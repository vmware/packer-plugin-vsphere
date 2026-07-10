// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package driver

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/vim25/types"
)

const (
	maxControllersPerType  = 4
	maxControllerBusNumber = 3

	// SCSI unit and disk limits (vSphere 8+).
	scsiReservedUnitNumber       = 7
	maxLSISCSIUnit               = 15
	maxPVSCSIUnit                = 64
	maxLSISCSIDisksPerController = 15
	maxPVSCSIDisksPerController  = 64

	// NVMe and SATA unit and disk limits.
	maxNVMeUnit               = 14
	maxSATAUnit               = 29
	maxNVMEDisksPerController = maxNVMeUnit + 1
	maxSATADisksPerController = maxSATAUnit + 1

	maxSCSIDisksPerVM    = 256
	maxLSISCSIDisksPerVM = 60
	maxSATADisksPerVM    = 120
	maxNVMEDisksPerVM    = 60

	controllerTypeLSILogic    = "lsilogic"
	controllerTypeLSILogicSAS = "lsilogic-sas"
	controllerTypePVSCSI      = "pvscsi"
	controllerTypeNVMe        = "nvme"
	controllerTypeSCSI        = "scsi"
	controllerTypeSATA        = "sata"
)

var controllerTypeDisplayNames = map[string]string{
	controllerTypeLSILogic:    "LSI Logic",
	controllerTypeLSILogicSAS: "LSI Logic SAS",
	controllerTypePVSCSI:      "PVSCSI",
	controllerTypeNVMe:        "NVMe",
	controllerTypeSATA:        "SATA",
}

var controllerUnitPattern = regexp.MustCompile(`^(scsi|nvme|sata)([0-3]):([0-9]+)$`)

// SupportedDiskControllerTypes lists valid disk_controller_type values.
var SupportedDiskControllerTypes = map[string]struct{}{
	controllerTypeLSILogic:    {},
	controllerTypeLSILogicSAS: {},
	controllerTypePVSCSI:      {},
	controllerTypeNVMe:        {},
	controllerTypeSCSI:        {},
	controllerTypeSATA:        {},
}

// ControllerKind identifies a storage controller bus family.
type ControllerKind int

const (
	ControllerKindSCSI ControllerKind = iota
	ControllerKindNVMe
	ControllerKindSATA
)

// ControllerAddress is a parsed controller:unit address (e.g. scsi0:1).
type ControllerAddress struct {
	Kind ControllerKind
	Bus  int
	Unit int
}

func (a ControllerAddress) String() string {
	return fmt.Sprintf("%s%d:%d", controllerKindPrefix(a.Kind), a.Bus, a.Unit)
}

func controllerKindPrefix(kind ControllerKind) string {
	switch kind {
	case ControllerKindNVMe:
		return "nvme"
	case ControllerKindSATA:
		return "sata"
	default:
		return "scsi"
	}
}

// ParseControllerUnit parses an address such as "scsi0:1" or "nvme0:0".
func ParseControllerUnit(raw string) (ControllerAddress, error) {
	matches := controllerUnitPattern.FindStringSubmatch(strings.TrimSpace(raw))
	if matches == nil {
		return ControllerAddress{}, fmt.Errorf("invalid disk_controller_unit %q: expected format scsi0:1, nvme0:0, or sata0:2", raw)
	}

	bus, err := strconv.Atoi(matches[2])
	if err != nil {
		return ControllerAddress{}, fmt.Errorf("invalid disk_controller_unit %q: bus number must be 0-3", raw)
	}

	unit, err := strconv.Atoi(matches[3])
	if err != nil {
		return ControllerAddress{}, fmt.Errorf("invalid disk_controller_unit %q: unit must be a non-negative integer", raw)
	}

	var kind ControllerKind
	switch matches[1] {
	case "scsi":
		kind = ControllerKindSCSI
	case "nvme":
		kind = ControllerKindNVMe
	case "sata":
		kind = ControllerKindSATA
	}

	return ControllerAddress{Kind: kind, Bus: bus, Unit: unit}, nil
}

// ValidateControllerUnitStatic validates address format and static vSphere limits.
func ValidateControllerUnitStatic(raw string) error {
	return ValidateControllerUnitStaticForConfig(raw, nil, 0)
}

// ValidateControllerUnitStaticForConfig validates unit limits using disk_controller_type hints.
// typeIndex is the effective disk_controller_type index that would create the controller for
// this address's bus if it does not already exist on the template. Callers are responsible for
// resolving typeIndex so that multiple addresses sharing the same bus map to the same index,
// matching the runtime allocator in AddStorageDevices.
func ValidateControllerUnitStaticForConfig(raw string, diskControllerTypes []string, typeIndex int) error {
	addr, err := ParseControllerUnit(raw)
	if err != nil {
		return err
	}

	if addr.Bus < 0 || addr.Bus > maxControllerBusNumber {
		return fmt.Errorf("invalid disk_controller_unit %q: bus number must be 0-%d", raw, maxControllerBusNumber)
	}

	if addr.Kind == ControllerKindSCSI && addr.Unit == scsiReservedUnitNumber {
		return fmt.Errorf("unit %d is reserved for the SCSI controller on %q", scsiReservedUnitNumber, raw)
	}

	// disk_controller_type entries are consumed in the order distinct new
	// buses are first referenced, across all controller kinds (SCSI, SATA,
	// and NVMe share one index sequence). If the entry landing on this
	// address's index is the wrong family for its bus kind, AddStorageDevices
	// will still refuse to create the mismatched controller at clone time,
	// but with a low-level error. Catch the ordering mistake here instead,
	// where it can be reported clearly and before any resources are touched.
	if typeIndex < len(diskControllerTypes) {
		if entryKind := controllerKindForType(diskControllerTypes[typeIndex]); entryKind != addr.Kind {
			return fmt.Errorf(
				"disk_controller_unit %q would create a new %s%d controller, but disk_controller_type[%d] is %q (%s). "+
					"disk_controller_type entries are consumed in the order distinct new buses are first referenced across storage blocks, "+
					"regardless of controller kind: reorder disk_controller_type so index %d holds a %s controller type, or reorder the storage blocks",
				raw, controllerKindPrefix(addr.Kind), addr.Bus, typeIndex, diskControllerTypes[typeIndex],
				displayNameForControllerType(diskControllerTypes[typeIndex]), typeIndex, strings.ToUpper(controllerKindPrefix(addr.Kind)),
			)
		}
	}

	maxUnit := staticMaxUnitForAddress(addr, diskControllerTypes, typeIndex)
	if addr.Unit < 0 || addr.Unit > maxUnit {
		displayName := staticDisplayNameForAddress(addr, diskControllerTypes, typeIndex)
		return fmt.Errorf("invalid disk_controller_unit %q: unit must be between 0 and %d for %s controllers", raw, maxUnit, displayName)
	}

	return nil
}

func staticDisplayNameForAddress(addr ControllerAddress, diskControllerTypes []string, typeIndex int) string {
	switch addr.Kind {
	case ControllerKindNVMe:
		return displayNameForControllerType(controllerTypeNVMe)
	case ControllerKindSATA:
		return displayNameForControllerType(controllerTypeSATA)
	default:
		if typeIndex < len(diskControllerTypes) {
			return displayNameForControllerType(diskControllerTypes[typeIndex])
		}
		if containsPVSCSIType(diskControllerTypes) {
			return displayNameForControllerType(controllerTypePVSCSI)
		}
		return "SCSI"
	}
}

func staticMaxUnitForAddress(addr ControllerAddress, diskControllerTypes []string, typeIndex int) int {
	if typeIndex < len(diskControllerTypes) {
		return maxUnitForControllerTypeString(diskControllerTypes[typeIndex], addr.Kind)
	}

	if addr.Kind == ControllerKindSCSI && containsPVSCSIType(diskControllerTypes) {
		return maxPVSCSIUnit
	}

	maxUnit, _ := maxUnitForKind(addr.Kind, nil)
	return maxUnit
}

func containsPVSCSIType(diskControllerTypes []string) bool {
	for _, controllerType := range diskControllerTypes {
		if controllerType == controllerTypePVSCSI {
			return true
		}
	}
	return false
}

func maxUnitForControllerTypeString(controllerType string, kind ControllerKind) int {
	switch kind {
	case ControllerKindNVMe:
		return maxNVMeUnit
	case ControllerKindSATA:
		return maxSATAUnit
	default:
		if controllerType == controllerTypePVSCSI {
			return maxPVSCSIUnit
		}
		return maxLSISCSIUnit
	}
}

func maxUnitForKind(kind ControllerKind, controller types.BaseVirtualController) (int, error) {
	switch kind {
	case ControllerKindSCSI:
		if _, ok := controller.(types.BaseVirtualSCSIController); ok && isPVSCSIController(controller) {
			return maxPVSCSIUnit, nil
		}
		return maxLSISCSIUnit, nil
	case ControllerKindNVMe:
		return maxNVMeUnit, nil
	case ControllerKindSATA:
		return maxSATAUnit, nil
	default:
		return 0, fmt.Errorf("unknown controller kind")
	}
}

func maxDisksPerController(controller types.BaseVirtualController) int {
	maxUnit, err := maxUnitForKind(controllerKindForController(controller), controller)
	if err != nil {
		return 0
	}
	if _, ok := controller.(types.BaseVirtualSCSIController); ok {
		if maxUnit == maxPVSCSIUnit {
			return maxPVSCSIDisksPerController
		}
		return maxLSISCSIDisksPerController
	}
	return maxUnit + 1
}

func controllerKindForController(controller types.BaseVirtualController) ControllerKind {
	kind, _, ok := controllerBusNumber(controller)
	if !ok {
		return ControllerKindSCSI
	}
	return kind
}

// displayNameForControllerType returns a user-facing label for a disk_controller_type value.
func displayNameForControllerType(controllerType string) string {
	if name, ok := controllerTypeDisplayNames[controllerType]; ok {
		return name
	}
	return "SCSI"
}

// controllerDisplayName returns a user-facing label for a runtime controller device.
func controllerDisplayName(controller types.BaseVirtualController) string {
	if isPVSCSIController(controller) {
		return "PVSCSI"
	}
	switch controller.(type) {
	case *types.VirtualLsiLogicSASController:
		return "LSI Logic SAS"
	case *types.VirtualLsiLogicController:
		return "LSI Logic"
	case *types.VirtualBusLogicController:
		return "BusLogic"
	case *types.VirtualNVMEController:
		return "NVMe"
	case types.BaseVirtualSATAController:
		return "SATA"
	default:
		return "SCSI"
	}
}

func isPVSCSIController(controller types.BaseVirtualController) bool {
	switch controller.(type) {
	case *types.ParaVirtualSCSIController:
		return true
	default:
		return false
	}
}

func controllerBusNumber(controller types.BaseVirtualController) (ControllerKind, int, bool) {
	switch c := controller.(type) {
	case types.BaseVirtualSCSIController:
		return ControllerKindSCSI, int(c.GetVirtualSCSIController().BusNumber), true
	case *types.VirtualNVMEController:
		return ControllerKindNVMe, int(c.BusNumber), true
	case types.BaseVirtualSATAController:
		return ControllerKindSATA, int(c.GetVirtualSATAController().BusNumber), true
	default:
		return 0, 0, false
	}
}

func controllerAddressLabel(controller types.BaseVirtualController) string {
	kind, bus, ok := controllerBusNumber(controller)
	if !ok {
		return "unknown"
	}
	return fmt.Sprintf("%s%d", controllerKindPrefix(kind), bus)
}

// FindControllerByBus returns the controller matching kind and bus number.
func FindControllerByBus(devices object.VirtualDeviceList, kind ControllerKind, bus int) types.BaseVirtualController {
	for _, device := range devices {
		controller, ok := device.(types.BaseVirtualController)
		if !ok {
			continue
		}
		k, b, ok := controllerBusNumber(controller)
		if ok && k == kind && b == bus {
			return controller
		}
	}
	return nil
}

// ListControllerAddresses returns sorted controller addresses present in devices.
func ListControllerAddresses(devices object.VirtualDeviceList) []string {
	var addrs []string
	for _, device := range devices {
		controller, ok := device.(types.BaseVirtualController)
		if !ok {
			continue
		}
		addrs = append(addrs, controllerAddressLabel(controller))
	}
	sort.Strings(addrs)
	return addrs
}

// OccupiedUnits returns unit numbers in use on a controller, including reserved units.
func OccupiedUnits(devices object.VirtualDeviceList, controller types.BaseVirtualController) map[int]bool {
	occupied := map[int]bool{}
	key := controller.GetVirtualController().Key

	if sc, ok := controller.(types.BaseVirtualSCSIController); ok {
		occupied[int(sc.GetVirtualSCSIController().ScsiCtlrUnitNumber)] = true
	}

	for _, device := range devices {
		d := device.GetVirtualDevice()
		if d.ControllerKey != key || d.UnitNumber == nil {
			continue
		}
		occupied[int(*d.UnitNumber)] = true
	}

	return occupied
}

func listAvailableUnits(devices object.VirtualDeviceList, controller types.BaseVirtualController) []int {
	maxUnit, err := maxUnitForKind(controllerKindForController(controller), controller)
	if err != nil {
		return nil
	}

	occupied := OccupiedUnits(devices, controller)
	var available []int
	for unit := 0; unit <= maxUnit; unit++ {
		if _, ok := controller.(types.BaseVirtualSCSIController); ok && unit == scsiReservedUnitNumber {
			continue
		}
		if !occupied[unit] {
			available = append(available, unit)
		}
	}
	return available
}

func formatAvailableUnits(units []int) string {
	if len(units) == 0 {
		return "none"
	}

	var ranges []string
	start := units[0]
	prev := units[0]

	for i := 1; i < len(units); i++ {
		if units[i] == prev+1 {
			prev = units[i]
			continue
		}
		ranges = append(ranges, formatUnitRange(start, prev))
		start = units[i]
		prev = units[i]
	}
	ranges = append(ranges, formatUnitRange(start, prev))
	return strings.Join(ranges, ", ")
}

func formatUnitRange(start, end int) string {
	if start == end {
		return strconv.Itoa(start)
	}
	return fmt.Sprintf("%d-%d", start, end)
}

func countDisksOnController(devices object.VirtualDeviceList, controller types.BaseVirtualController) int {
	key := controller.GetVirtualController().Key
	count := 0
	for _, device := range devices {
		d := device.GetVirtualDevice()
		if _, ok := device.(*types.VirtualDisk); !ok {
			continue
		}
		if d.ControllerKey == key {
			count++
		}
	}
	return count
}

func scsiDiskUnitRange(maxUnit int) string {
	return fmt.Sprintf("units 0-6, 8-%d; unit %d reserved", maxUnit, scsiReservedUnitNumber)
}

func validateControllerDiskCapacity(devices object.VirtualDeviceList, controller types.BaseVirtualController, adding int) error {
	current := countDisksOnController(devices, controller)
	requested := current + adding
	maxDisks := maxDisksPerController(controller)

	if requested <= maxDisks {
		return nil
	}

	label := controllerAddressLabel(controller)
	name := controllerDisplayName(controller)
	if isPVSCSIController(controller) {
		return fmt.Errorf("%s controller %s supports maximum %d disks (%s). Requested: %d disks",
			name, label, maxPVSCSIDisksPerController, scsiDiskUnitRange(maxPVSCSIUnit), requested)
	}
	if _, ok := controller.(types.BaseVirtualSCSIController); ok {
		return fmt.Errorf("%s controller %s supports maximum %d disks (%s). Requested: %d disks",
			name, label, maxLSISCSIDisksPerController, scsiDiskUnitRange(maxLSISCSIUnit), requested)
	}
	return fmt.Errorf("%s controller %s supports maximum %d disks. Requested: %d disks", name, label, maxDisks, requested)
}

func countVirtualDisksByKind(devices object.VirtualDeviceList) map[ControllerKind]int {
	counts := map[ControllerKind]int{}
	for _, device := range devices {
		if _, ok := device.(*types.VirtualDisk); !ok {
			continue
		}
		controllerKey := device.GetVirtualDevice().ControllerKey
		for _, controllerDevice := range devices {
			controller, ok := controllerDevice.(types.BaseVirtualController)
			if !ok {
				continue
			}
			if controller.GetVirtualController().Key != controllerKey {
				continue
			}
			kind, _, ok := controllerBusNumber(controller)
			if ok {
				counts[kind]++
			}
			break
		}
	}
	return counts
}

func hasPVSCSIControllerOnVM(devices object.VirtualDeviceList) bool {
	for _, device := range devices {
		controller, ok := device.(types.BaseVirtualController)
		if !ok {
			continue
		}
		if isPVSCSIController(controller) {
			return true
		}
	}
	return false
}

func validateAggregateStorageCapacity(devices object.VirtualDeviceList) error {
	counts := countVirtualDisksByKind(devices)

	if counts[ControllerKindSATA] > maxSATADisksPerVM {
		return fmt.Errorf("virtual machine supports maximum %d SATA disks. Requested: %d disks", maxSATADisksPerVM, counts[ControllerKindSATA])
	}
	if counts[ControllerKindNVMe] > maxNVMEDisksPerVM {
		return fmt.Errorf("virtual machine supports maximum %d NVMe disks. Requested: %d disks", maxNVMEDisksPerVM, counts[ControllerKindNVMe])
	}

	scsiMax := maxLSISCSIDisksPerVM
	if hasPVSCSIControllerOnVM(devices) {
		scsiMax = maxSCSIDisksPerVM
	}
	if counts[ControllerKindSCSI] > scsiMax {
		if scsiMax == maxSCSIDisksPerVM {
			return fmt.Errorf("virtual machine supports maximum %d SCSI disks with PVSCSI controllers. Requested: %d disks", maxSCSIDisksPerVM, counts[ControllerKindSCSI])
		}
		return fmt.Errorf("virtual machine supports maximum %d SCSI disks. Requested: %d disks", maxLSISCSIDisksPerVM, counts[ControllerKindSCSI])
	}

	return nil
}

// ValidateControllerUnitRuntime checks unit availability on an existing controller at clone time.
func ValidateControllerUnitRuntime(devices object.VirtualDeviceList, raw string, pending map[string]struct{}) error {
	addr, err := ParseControllerUnit(raw)
	if err != nil {
		return err
	}

	if pending != nil {
		if _, exists := pending[raw]; exists {
			return fmt.Errorf("unit %q is already assigned. Each unit can only be used once", raw)
		}
	}

	controller := FindControllerByBus(devices, addr.Kind, addr.Bus)
	if controller == nil {
		return nil
	}

	maxUnit, err := maxUnitForKind(addr.Kind, controller)
	if err != nil {
		return err
	}
	if addr.Unit > maxUnit {
		return fmt.Errorf("invalid disk_controller_unit %q: unit exceeds maximum %d for this controller type", raw, maxUnit)
	}

	if OccupiedUnits(devices, controller)[addr.Unit] {
		available := formatAvailableUnits(listAvailableUnits(devices, controller))
		return fmt.Errorf("unit %q is already in use. Available units: %s", addr.String(), available)
	}

	return nil
}

// ValidateDiskControllerTypes returns an error if any type is unsupported.
func ValidateDiskControllerTypes(types []string) error {
	for i, controllerType := range types {
		if _, ok := SupportedDiskControllerTypes[controllerType]; !ok {
			return fmt.Errorf("unsupported controller type %q at disk_controller_type[%d]. Supported types: lsilogic, lsilogic-sas, pvscsi, nvme, scsi, sata", controllerType, i)
		}
	}
	return nil
}

func controllerKindForType(controllerType string) ControllerKind {
	switch controllerType {
	case controllerTypeNVMe:
		return ControllerKindNVMe
	case controllerTypeSATA:
		return ControllerKindSATA
	default:
		return ControllerKindSCSI
	}
}

func countControllersByKind(devices object.VirtualDeviceList) map[ControllerKind]int {
	counts := map[ControllerKind]int{}
	for _, device := range devices {
		controller, ok := device.(types.BaseVirtualController)
		if !ok {
			continue
		}
		kind, _, ok := controllerBusNumber(controller)
		if ok {
			counts[kind]++
		}
	}
	return counts
}

func validateControllerCount(devices object.VirtualDeviceList, kind ControllerKind, adding int) error {
	counts := countControllersByKind(devices)
	if counts[kind]+adding > maxControllersPerType {
		return fmt.Errorf("maximum of %d %s controllers allowed per virtual machine", maxControllersPerType, controllerKindPrefix(kind))
	}
	return nil
}

func createController(devices object.VirtualDeviceList, controllerType string) (types.BaseVirtualDevice, error) {
	switch controllerType {
	case controllerTypeNVMe:
		return devices.CreateNVMEController()
	case controllerTypeSATA:
		return devices.CreateSATAController()
	default:
		return devices.CreateSCSIController(controllerType)
	}
}

func assignDiskAtUnit(devices object.VirtualDeviceList, disk *types.VirtualDisk, controller types.BaseVirtualController, unit int32, linkController bool) {
	d := disk.GetVirtualDevice()
	d.ControllerKey = controller.GetVirtualController().Key
	d.UnitNumber = &unit
	if d.Key == 0 {
		d.Key = devices.NewKey()
	}
	if linkController {
		controller.GetVirtualController().Device = append(controller.GetVirtualController().Device, d.Key)
	}
}

func controllerNotFoundError(raw string, available []string) error {
	addr, err := ParseControllerUnit(raw)
	if err != nil {
		return err
	}
	controllerLabel := fmt.Sprintf("%s%d", controllerKindPrefix(addr.Kind), addr.Bus)
	if len(available) == 0 {
		return fmt.Errorf("controller %q not found: no storage controllers on template", controllerLabel)
	}
	return fmt.Errorf("controller %q not found. Available controllers: %s", controllerLabel, strings.Join(available, ", "))
}

func typePoolExhaustedError(raw string) error {
	addr, err := ParseControllerUnit(raw)
	if err != nil {
		return err
	}
	controllerLabel := fmt.Sprintf("%s%d", controllerKindPrefix(addr.Kind), addr.Bus)
	return fmt.Errorf("no disk_controller_type entries remain to create controller %q. Add an entry to disk_controller_type or reference an existing controller", controllerLabel)
}
