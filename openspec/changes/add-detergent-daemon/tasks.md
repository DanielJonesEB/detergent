# Implementation Tasks

## 1. Core Infrastructure
- [ ] 1.1 Set up project structure (Go module, CLI framework)
- [ ] 1.2 Implement YAML configuration parser
- [ ] 1.3 Implement concern graph validation (cycle detection, reference validation)

## 2. Git Operations
- [ ] 2.1 Implement worktree management (create, list, cleanup)
- [ ] 2.2 Implement branch operations (create, fast-forward, push)
- [ ] 2.3 Implement commit message formatting with concern tags
- [ ] 2.4 Implement git notes for no-change audit trail

## 3. Daemon Core
- [ ] 3.1 Implement branch watching (poll for new commits)
- [ ] 3.2 Implement context assembly (diffs + commit messages)
- [ ] 3.3 Implement agent invocation interface
- [ ] 3.4 Implement main loop with error handling and retry

## 4. CLI Commands
- [ ] 4.1 Implement `detergent run` (start daemon)
- [ ] 4.2 Implement `detergent status` (graph visualization)
- [ ] 4.3 Implement `detergent viz` (static graph display)

## 5. Testing & Documentation
- [ ] 5.1 Write integration tests with mock agent
- [ ] 5.2 Write unit tests for graph validation
- [ ] 5.3 Create example configuration files
- [ ] 5.4 Write usage documentation
