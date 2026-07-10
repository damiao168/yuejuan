# Story Workflow

Each Grading Agent Lab story is a small delivery unit with explicit evidence.

## Required Stages

1. `spec-v1`: initial business and engineering specification.
2. `spec-review`: risks, missing cases, and unclear terms.
3. `spec-v2`: revised specification after review.
4. `implementation`: code, data, or documentation created under `lab`.
5. `implementation-review`: defects found by inspection or tests.
6. `implementation-fixes`: changes made after review.
7. `next-story-entry`: the story can close only when verification is recorded.

## Done Means

- The artifact lives under `lab`.
- The story links to its implementation files.
- Tests or manual checks are listed.
- Any remaining limitation is explicit and does not pretend to be production capability.
