package github

import (
	"list"
)

// The trybot workflow.
workflows: trybot: _repo.bashWorkflow & {
	on: {
		push: {
			branches: list.Concat([[_repo.testDefaultBranch], _repo.protectedBranchPatterns]) // do not run PR branches
		}
		pull_request: {}
	}

	jobs: {
		test: {
			"runs-on": _repo.linuxMachine

			let installGo = _repo.installGo & {
				#setupGo: with: "go-version": _repo.latestGo
				_
			}

			// Only run the trybot workflow if we have the trybot trailer, or
			// if we have no special trailers. Note this condition applies
			// after and in addition to the "on" condition above.
			if: "\(_repo.containsTrybotTrailer) || ! \(_repo.containsDispatchTrailer)"

			steps: [
				for v in _repo.checkoutCode {v},
				for v in installGo {v},
				for v in _repo.setupCaches {v},

				_repo.earlyChecks,

				{
					name: "Verify"
					run:  "go mod verify"
				},
				{
					name: "Generate"
					run:  "go generate ./..."
				},
				{
					name: "Check CUE formatting"
					run:  "go tool cue fmt --files --check ."
				},
				{
					name: "Test"
					run:  "go test ./..."
				},
				{
					name: "Race test"
					run:  "go test -race ./..."
					if:   "github.repository == '\(_repo.githubRepositoryPath)' && (\(_repo.isProtectedBranch) || \(_repo.isTestDefaultBranch))"
				},
				_repo.staticcheck,
				_repo.goChecks,

				_repo.checkGitClean,
			]
		}
	}
}
