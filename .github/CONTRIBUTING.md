# Contributing Guidelines

We greatly value feedback and contributions from our community.

Please review this document before submitting any issues or pull requests to ensure we
have all the necessary information to effectively collaborate on your contribution.

> [!TIP]
If you need help or have questions about using the plugin, please refer to the
[documentation](https://github.com/vmware/packer-plugin-vsphere/tree/main/docs) or open
a [discussion][gh-discussions].

## Issues

Use [GitHub issues][gh-issues] to report bugs or suggest enhancements using the
following guidelines.

> [!WARNING]
> Issues that do not follow the guidelines may be closed by the maintainers without
> further investigation.

Before opening an issue, please [search existing issues](https://github.com/vmware/packer-plugin-vsphere/issues?q=is%3Aissue+is%3Aopen+label%3Abug)
to avoid duplicates.

When opening an issue, use the provided issue form to ensure that you provide all the
necessary details. These details are important for maintainers to understand and
reproduce the issue.

> [!IMPORTANT]
> - Ensure that you are using a recent version of the plugin.
> - Ensure that you are using a supported version of VMware vSphere. The plugin supports versions in accordance with the [Broadcom Product Lifecycle][product-lifecycle].

> [!TIP]
> - Learn about [formatting code on GitHub](https://docs.github.com/en/get-started/writing-on-github/getting-started-with-writing-and-formatting-on-github/basic-writing-and-formatting-syntax#quoting-code).
> - Learn about [referencing issues](https://docs.github.com/en/get-started/writing-on-github/getting-started-with-writing-and-formatting-on-github/basic-writing-and-formatting-syntax#referencing-issues-and-pull-requests).
> - Learn about [creating a GitHub Gist](https://docs.github.com/en/get-started/writing-on-github/editing-and-sharing-content-with-gists/creating-gists).

## Pull Requests

Use GitHub pull requests to propose changes to the codebase using the following guidelines.

> [!WARNING]
> Pull requests that do not follow the guidelines may be closed by the maintainers
> without further review.

**Before** submitting a pull request, ensure that:

1. You have [opened a discussion][gh-discussions] to discuss any **significant** work with
   the maintainer(s). This ensures that your contribution is aligned with the
   project's direction and avoids unnecessary work.
2. You have identified or [open an issue][gh-issues]. This ensures that your contribution
   focuses on a specific topic and avoids duplicating effort.
3. You have forked the repository. Refer to the [GitHub documentation][gh-forks] for help.
3. You are working against the latest source on the `main` branch. You may need to
   rebase your branch against the latest `main` branch.
4. You have created a topic branch based on `main`. Do not work directly on the `main` branch.
5. You have modified the source based on logical units of work. Focus on the specific change
   you are contributing. Pull requests that contain multiple unrelated changes will be
   rejected.
4. You have followed the existing style and conventions of the project. 
5. You have added tests for your changes.
5. You have generated the updated documentation and associated assets by running `make generate`.
5. You have tested building the plugin by running `make build`.
7. You have tested your changes with a local build of the plugin by running `make dev`.
9. You have verified all new and existing tests are passing by running `make test`.
10. You have used [Conventional Commits][conventional-commits] format for commit messages.
11. You have signed-off and committed your changes [using clear commit messages][git-commit].

When opening a pull request, ensure that:

1. You title your pull request using the [Conventional Commits][conventional-commits] format.
2. You provide a detailed description of the changes in the pull request template.
2. You open any work-in-progress pull requests as a draft.
3. You mark the pull request as ready for review when you are ready for it to be reviewed.
4. You follow the status checks for the pull request to ensure that all checks are passing. 
5. You stay involved in the conversation with the maintainers to ensure that your contribution
   can be reviewed.

> [!TIP]
> If you have any questions about the contribution process, open a [discussion][gh-discussions].

### Contributor Flow

This is an outline of the contributor workflow:

Example:

```shell
git remote add upstream https://github.com/<org-name>/<repo-name>.git
git checkout -b feat/add-x main
git commit --signoff --message "feat: add support for x
  Added support for x.

  Signed-off-by: Jane Doe <jdoe@example.com>

  Ref: #123"
git push origin feat/add-x
```

### Formatting Commit Messages

Follow the conventions on [How to Write a Git Commit Message][git-commit] and use
[Conventional Commits][conventional-commits].

Be sure to include any related GitHub issue references in the commit message.

Example:

```markdown
feat: add support for x

Added support for x.

Signed-off-by: Jane Doe <jdoe@example.com>

Ref: #123
```

### Stay In Sync With Upstream

When your branch gets out of sync with the `upstream/main` branch, use the
following to update:

```shell
git checkout feat/add-x
git fetch --all
git pull --rebase upstream main
git push --force-with-lease origin feat/add-x
```

### Updating Pull Requests

If your pull request fails to pass or needs changes based on code review, you'll
most likely want to squash these changes into existing commits.

If your pull request contains a single commit or your changes are related to the
most recent commit, you can simply amend the commit.

```shell
git add .
git commit --amend
git push --force-with-lease origin feat/add-x
```

If you need to squash changes into an earlier commit, you can use:

```shell
git add .
git commit --fixup <commit>
git rebase --interactive --autosquash upstream/main
git push --force-with-lease origin feat/add-x
```

When resolving review comments, mark the conversation as resolved and note the commit
SHA that addresses the review comment. This helps maintainers verify the issue has been
resolved.

Request a review from the maintainers when you are ready for a follow-up review.

[conventional-commits]: https://conventionalcommits.org
[gh-discussions]: https://github.com/vmware/packer-plugin-vsphere/discussions
[gh-forks]: https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/working-with-forks/fork-a-repo
[gh-issues]: https://github.com/vmware/packer-plugin-vsphere/issues
[gh-pull-requests]: https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/proposing-changes-to-your-work-with-pull-requests/creating-a-pull-request
[git-commit]: https://cbea.ms/git-commit
[product-lifecycle]: https://support.broadcom.com/group/ecx/productlifecycle
