// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

//go:generate packer-sdc struct-markdown
//go:generate packer-sdc mapstructure-to-hcl2 -type ImportImageConfig

package supervisor

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	imgregv1 "github.com/vmware-tanzu/image-registry-operator-api/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ImportTargetKind       = "ContentLibrary"
	ImportTargetAPIVersion = "imageregistry.vmware.com/v1alpha1"

	DefaultWatchImageImportTimeoutSec = 600

	ImportRequestDefaultNamePrefix = "packer-vsphere-supervisor-import-req-"

	StateKeyImageImportRequestCreated = "item_import_req_created"
	StateKeyImportedImageName         = "imported_image_name"

	importOVFFeatureNotEnabledMsg = "WCP_VMImageService_ImportOVF feature is not enabled"
)

type ImportImageConfig struct {
	// The remote URL where the to-be-imported image is hosted.
	ImportSourceURL string `mapstructure:"import_source_url"`
	// The SSL certificate of the remote HTTP server that hosts the to-be-imported image.
	ImportSourceSSLCertificate string `mapstructure:"import_source_ssl_certificate"`
	// Name of a writable and import-allowed ContentLibrary resource in the namespace where the image will be imported.
	ImportTargetLocationName string `mapstructure:"import_target_location_name"`
	// The type of imported image.
	// Defaults to suffix of the source URL. Available options include `ovf` and `iso`.
	ImportTargetImageType string `mapstructure:"import_target_image_type"`
	// Name of the imported image.
	// Defaults to the file name of the image referenced in the source URL.
	ImportTargetImageName string `mapstructure:"import_target_image_name"`
	// The name of the image import request.
	// Defaults to `packer-vsphere-supervisor-import-req-<random-suffix>`.
	ImportRequestName string `mapstructure:"import_request_name"`
	// The timeout in seconds to wait for the image to be imported.
	// Defaults to `600`.
	WatchImportTimeoutSec int `mapstructure:"watch_import_timeout_sec"`
	// Preserve the import request in the Supervisor cluster after the build finishes.
	// Defaults to `false`.
	KeepImportRequest bool `mapstructure:"keep_import_request"`
	// Clean the imported image after the build finishes. If set to `true`, the imported image will be deleted.
	// Defaults to `false`.
	CleanImportedImage bool `mapstructure:"clean_imported_image"`
}

func (c *ImportImageConfig) Prepare() []error {
	if c.ImportSourceURL == "" {
		return nil
	}

	var errs []error
	if c.ImportTargetLocationName == "" {
		errs = append(errs, fmt.Errorf("config import_target_location_name is required for importing image"))
	}

	if strings.HasPrefix(c.ImportSourceURL, "https://") && c.ImportSourceSSLCertificate == "" {
		errs = append(errs, fmt.Errorf("config import_source_ssl_certificate is required for https based source urls"))
	}

	switch c.ImportTargetImageType {
	case "":
		if strings.HasSuffix(c.ImportSourceURL, ".iso") {
			c.ImportTargetImageType = "iso"
		} else if strings.HasSuffix(c.ImportSourceURL, ".ovf") || strings.HasSuffix(c.ImportSourceURL, ".ova") {
			c.ImportTargetImageType = "ovf"
		} else {
			errs = append(errs, fmt.Errorf("cannot infer supported image type from source url %s", c.ImportSourceURL))
		}
	case "ovf", "iso":
		// If it's already "ovf" or "iso", do nothing.
	default:
		errs = append(errs, fmt.Errorf("unsupported ImportTargetImageType: %s", c.ImportTargetImageType))
	}

	if c.WatchImportTimeoutSec == 0 {
		c.WatchImportTimeoutSec = DefaultWatchImageImportTimeoutSec
	}
	if c.ImportRequestName == "" {
		c.ImportRequestName = ImportRequestDefaultNamePrefix + uuid.NewString()[:5]
	}

	return errs
}

type StepImportImage struct {
	ImportImageConfig *ImportImageConfig

	ImportItemResourceName, Namespace string
	TargetItemType                    imgregv1.ContentLibraryItemType
	KubeWatchClient                   client.WithWatch

	// The lock and bool are mainly used to help testing.
	Mu                    sync.Mutex
	IsWatchingImageImport bool
}

func (s *StepImportImage) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	logger := state.Get("logger").(*PackerLogger)

	var err error
	defer func() {
		if err != nil {
			state.Put("error", err)
		}
	}()

	if err = s.initStep(state, logger); err != nil {
		logger.Error("failed to initialize image import: %s", err.Error())
		return multistep.ActionHalt
	}

	if err = s.validate(ctx, logger); err != nil {
		logger.Error("failed to validate import image configs: %s", err.Error())
		return multistep.ActionHalt
	}

	logger.Info("Importing the source image from %s to %s.",
		s.ImportImageConfig.ImportSourceURL, s.ImportImageConfig.ImportTargetLocationName)

	if err = s.createImageImportRequest(ctx, logger); err != nil {
		return multistep.ActionHalt
	}
	state.Put(StateKeyImageImportRequestCreated, true)

	if err = s.watchItemImport(ctx, state, logger); err != nil {
		return multistep.ActionHalt
	}

	logger.Info("Finished importing the image from %s to %s.",
		s.ImportImageConfig.ImportSourceURL, s.ImportImageConfig.ImportTargetLocationName)

	return multistep.ActionContinue
}

func (s *StepImportImage) validate(ctx context.Context, logger *PackerLogger) error {
	logger.Info("Validating image import request...")

	var err error
	if err = s.checkImportFeatureEnabled(ctx, logger); err != nil {
		return err
	}

	if err = s.checkImportTarget(ctx, logger); err != nil {
		return err
	}

	logger.Info("Image import configs are valid.")
	return nil
}

func (s *StepImportImage) Cleanup(state multistep.StateBag) {
	if v, ok := state.GetOk(StateKeyImageImportRequestCreated); !ok || !v.(bool) {
		// Either the image import step was skipped or the object was not created successfully.
		// Skip deleting the ContentLibraryItemImportRequest object.
		return
	}

	logger := state.Get("logger").(*PackerLogger)
	if s.ImportImageConfig.KeepImportRequest && !s.ImportImageConfig.CleanImportedImage {
		logger.Info("Skipping clean up of the ContentLibraryItemImportRequest object as specified in config.")
		return
	}

	if !s.ImportImageConfig.KeepImportRequest {
		logger.Info("Deleting the ContentLibraryItemImportRequest object %s in namespace %s.",
			s.ImportImageConfig.ImportRequestName, s.Namespace)
		ctx := context.Background()
		itemImportReqObj := &imgregv1.ContentLibraryItemImportRequest{
			ObjectMeta: metav1.ObjectMeta{
				Name:      s.ImportImageConfig.ImportRequestName,
				Namespace: s.Namespace,
			},
		}
		if err := s.KubeWatchClient.Delete(ctx, itemImportReqObj); err != nil {
			logger.Error("error deleting the ContentLibraryItemImportRequest object : %s", err)
		} else {
			logger.Info("Successfully deleted the ContentLibraryItemImportRequest object %s in namespace %s.",
				s.ImportImageConfig.ImportRequestName, s.Namespace)
		}
	}

	if s.ImportImageConfig.CleanImportedImage {
		if s.ImportItemResourceName == "" {
			return
		}

		// Clean imported image if the image is imported and clean image is set as true.
		logger.Info("Deleting the imported ContentLibraryItem object %s in namespace %s.",
			s.ImportItemResourceName, s.Namespace)
		importedImage := &imgregv1.ContentLibraryItem{
			ObjectMeta: metav1.ObjectMeta{
				Name:      s.ImportItemResourceName,
				Namespace: s.Namespace,
			},
		}
		ctx := context.Background()
		if err := s.KubeWatchClient.Delete(ctx, importedImage); err != nil {
			logger.Error("error deleting the ContentLibraryItem object %s: %s", s.ImportItemResourceName, err)
		} else {
			logger.Info("Successfully deleted the ContentLibraryItem object %s in namespace %s.",
				s.ImportItemResourceName, s.Namespace)
		}
	}
}

func (s *StepImportImage) initStep(state multistep.StateBag, logger *PackerLogger) error {
	if err := CheckRequiredStates(state,
		StateKeySupervisorNamespace,
		StateKeyKubeClient,
	); err != nil {
		logger.Error("error checking required states: %s", err)
		return err
	}

	var ok bool
	if s.Namespace, ok = state.Get(StateKeySupervisorNamespace).(string); !ok {
		return fmt.Errorf("failed to cast %s to type string", StateKeySupervisorNamespace)
	}
	if s.KubeWatchClient, ok = state.Get(StateKeyKubeClient).(client.WithWatch); !ok {
		return fmt.Errorf("failed to cast %s to type client.WithWatch", StateKeyKubeClient)
	}

	if s.ImportImageConfig.ImportTargetImageType != "" {
		s.TargetItemType = imgregv1.ContentLibraryItemType(strings.ToUpper(s.ImportImageConfig.ImportTargetImageType))
	}

	return nil
}

func (s *StepImportImage) checkImportFeatureEnabled(ctx context.Context, logger *PackerLogger) error {
	importReq := &imgregv1.ContentLibraryItemImportRequest{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    s.Namespace,
			GenerateName: "import-",
		},
	}

	// Use dry run mode to send an image import creation request to API-Server without applying the resource.
	if err := client.NewDryRunClient(s.KubeWatchClient).Create(ctx, importReq); err != nil && strings.Contains(err.Error(), importOVFFeatureNotEnabledMsg) {
		logger.Error("image import feature is not enabled")
		return err
	}

	return nil
}

func (s *StepImportImage) checkImportTarget(ctx context.Context, logger *PackerLogger) error {
	cl := &imgregv1.ContentLibrary{}
	objKey := client.ObjectKey{Name: s.ImportImageConfig.ImportTargetLocationName, Namespace: s.Namespace}
	if err := s.KubeWatchClient.Get(ctx, objKey, cl); err != nil {
		logger.Error("failed to return the content library by name %s in namespace %s",
			s.ImportImageConfig.ImportTargetLocationName, s.Namespace)
		return err
	}

	if !cl.Spec.Writable || !cl.Spec.AllowImport {
		return fmt.Errorf("import target content library %q is not writable or does not allow import",
			s.ImportImageConfig.ImportTargetLocationName)
	}

	if s.TargetItemType != imgregv1.ContentLibraryItemTypeOvf && s.TargetItemType != imgregv1.ContentLibraryItemTypeIso {
		return fmt.Errorf("image type %s is not supported", s.ImportImageConfig.ImportTargetImageType)
	}

	return nil
}

func (s *StepImportImage) createImageImportRequest(ctx context.Context, logger *PackerLogger) error {
	logger.Info("Creating ContentLibraryItemImportRequest object %s in namespace %s.", s.ImportImageConfig.ImportRequestName, s.Namespace)

	imageImportReq := &imgregv1.ContentLibraryItemImportRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.ImportImageConfig.ImportRequestName,
			Namespace: s.Namespace,
		},
		Spec: imgregv1.ContentLibraryItemImportRequestSpec{
			Source: imgregv1.ContentLibraryItemImportRequestSource{
				URL:            s.ImportImageConfig.ImportSourceURL,
				SSLCertificate: s.ImportImageConfig.ImportSourceSSLCertificate,
			},
			Target: imgregv1.ContentLibraryItemImportRequestTarget{
				Library: imgregv1.LocalObjectRef{
					Kind:       ImportTargetKind,
					APIVersion: ImportTargetAPIVersion,
					Name:       s.ImportImageConfig.ImportTargetLocationName,
				},
				Item: imgregv1.ContentLibraryItemImportRequestTargetItem{
					Name: s.ImportImageConfig.ImportTargetImageName,
					Type: s.TargetItemType,
				},
			},
		},
	}

	if err := s.KubeWatchClient.Create(ctx, imageImportReq); err != nil {
		logger.Error("error creating the ContentLibraryItemImportRequest object %s.", imageImportReq.Name)
		return err
	}

	logger.Info("Successfully created the ContentLibraryItemImportRequest object %s.", imageImportReq.Name)
	return nil
}

func (s *StepImportImage) watchItemImport(ctx context.Context, state multistep.StateBag, logger *PackerLogger) error {
	itemImportReqWatch, err := s.KubeWatchClient.Watch(ctx, &imgregv1.ContentLibraryItemImportRequestList{}, &client.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("metadata.name", s.ImportImageConfig.ImportRequestName),
		Namespace:     s.Namespace,
	})

	if err != nil {
		logger.Error("error watching the ContentLibraryItemImportRequest object in supervisor cluster")
		return err
	}

	timedCtx, cancel := context.WithTimeout(ctx, time.Duration(s.ImportImageConfig.WatchImportTimeoutSec)*time.Second)

	defer func() {
		itemImportReqWatch.Stop()
		cancel()

		s.Mu.Lock()
		s.IsWatchingImageImport = false
		s.Mu.Unlock()
	}()

	s.Mu.Lock()
	s.IsWatchingImageImport = true
	s.Mu.Unlock()

	logger.Info("Waiting for the image import request to complete...")

	for {
		select {
		case event := <-itemImportReqWatch.ResultChan():
			if event.Object == nil {
				return fmt.Errorf("watch ContentLibraryItemImportRequest event object is nil")
			}

			itemImportReqObj, ok := event.Object.(*imgregv1.ContentLibraryItemImportRequest)
			if !ok {
				return fmt.Errorf("failed to convert the watch ContentLibraryItemImportRequest event object")
			}

			if itemImportReqObj.Status.ItemRef != nil {
				s.ImportItemResourceName = itemImportReqObj.Status.ItemRef.Name
			}

			importSuccess := false
			for _, cond := range itemImportReqObj.Status.Conditions {
				if cond.Type == imgregv1.ContentLibraryItemImportRequestComplete {
					importSuccess = cond.Status == corev1.ConditionTrue
					break
				}
			}
			if importSuccess {
				// Set VM image ref name if the import is successful.
				state.Put(StateKeyImportedImageName, strings.Replace(s.ImportItemResourceName, "clitem-", "vmi-", 1))

				logger.Info("Successfully imported the image as a content library item %q.", itemImportReqObj.Status.ItemRef)
				return nil
			}

		case <-timedCtx.Done():
			return fmt.Errorf("timed out watching for ContentLibraryItemImportRequest object to complete")
		}
	}
}
