Written in Golang.

Tests in Ginkgo and Gomega.

All requirements in TRUTH.md must be proven with an outside-in end-to-end test written in Ginkgo/Gomega. It should compile the `line` binary, and invoke it across a process boundary.

The full expected user workflow should be tested, over the boundaries users would cross. So the end-to-end tests must not 'shortcut' by invoking `line` commands that would actually be triggered by Git hooks, a Git commit, or similar.

All such tests should take place in a completely separate temp directory, so as not to pollute this repo.

If necessary each test should set up whatever it needs, such as Git repos, `line.yaml`.

It is acceptable for multiple requirements to be proven in one test, but each must be clearly marked.

Use Beads for issue tracking.
