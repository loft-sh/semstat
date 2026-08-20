# semstat

`semstat` reports facts about semantic version strings. It parses them,
classifies them, and orders them. It never produces a new one: there is no bump,
no tag, no write of any kind.

> **Support.** This tool encodes vCluster release conventions, in particular the
> prerelease vocabulary in `semstat type`. It is public because our CI needs to
> download it without a token, not because it is a product. There is no support
> promise and no stability guarantee outside loft-sh CI.

## Why it exists

Release pipelines ask "is this version newer" in shell, and the usual answer is
`sort -V`, which is close enough to be dangerous. It ranks build metadata, which
the semver spec says must never affect precedence, and it sorts `v2.0.0-rc.2`
above `v2.0.0`, so a release candidate can block the release it was a candidate
for.

A GitHub Action cannot fix it, because those comparisons live inside bash loops
and shell functions where a workflow step cannot reach. A binary can.

Comparing is only half of it. Routing a tag needs to know that `-rc.2` is a
release candidate and `-next.internal.3` is not, and that a tag shaped like
neither must stop the pipeline rather than be guessed at.

## Install

```bash
brew install loft-sh/tap/semstat            # macOS
go install github.com/loft-sh/semstat@latest
```

Or download a binary for your platform from the
[releases page](https://github.com/loft-sh/semstat/releases), or run the image:

```bash
docker run --rm ghcr.io/loft-sh/semstat:latest type v2.0.0-rc.2
```

## Commands

```
semstat <command> [arguments]
```

Every version is an argument; nothing reads stdin. A leading `v` is optional and
never changes an answer. Ordering follows semver precedence, so build metadata
never affects it and a prerelease always sorts below its final release.

| Command | Prints | Exit codes |
| -- | -- | -- |
| `validate <version>` | nothing, reason on stderr when invalid | 0 valid, 1 invalid, 64 misuse |
| `parse <version>` | one line of JSON | 0, 2 unreadable, 64 misuse |
| `type <version>` | one word | 0, 2 unreadable or unroutable, 64 misuse |
| `compare <a> <b>` | `-1`, `0` or `1` | 0, 2 unreadable, 64 misuse |
| `gt <a> <b>` | nothing | 0 above, 1 not above, 2 unreadable, 64 misuse |
| `version` | semstat's own version | 0, 64 misuse |
| `help` | the usage text | 0, 64 misuse |

Misuse is always 64, on every command: an unknown command, the wrong number of
arguments, or an option where a version was expected. It never overlaps with 2, so accepting 2 as a fact about the version
never also accepts "that subcommand is not in this binary".

### validate

Is this a version at all? The answer is the exit code, so it reads as a plain
condition. Nothing goes to stdout; the reason goes to stderr.

```console
$ semstat validate v1.2.3
$ echo $?
0

$ semstat validate 1.2
semstat: "1.2" is not a valid semantic version: invalid semantic version
$ echo $?
1
```

It accepts any valid semver, including the suffixes `type` refuses, because "is
this parseable" and "is this a release we route" are different questions.

### parse

The fields, as one line of JSON. Every key is always present. `prerelease` and
`build` are `null` when absent rather than empty strings, so a caller can tell
"no prerelease" from "a prerelease I failed to read".

```console
$ semstat parse v1.2.3-rc.1+build.5
{"major":1,"minor":2,"patch":3,"prerelease":"rc.1","build":"build.5","raw":"v1.2.3-rc.1+build.5"}

$ semstat parse v1.2.3
{"major":1,"minor":2,"patch":3,"prerelease":null,"build":null,"raw":"v1.2.3"}
```

`raw` is the string you passed in, `v` and all, so a caller can echo the tag it
was given back to a user without rebuilding it.

### type

Which channel does this release belong to? One word: `stable`, `alpha`, `beta`,
`rc`, `next` or `next-internal`.

```console
$ semstat type v2.0.0
stable

$ semstat type v2.0.0-rc.2
rc

$ semstat type v1.2.3-next.internal.4
next-internal
```

A suffix outside that vocabulary is an error, not a sixth answer. This is the
one command that rejects versions `validate` accepts, and the counter is part of
the shape, so `-rc` and `-rc2` fail where `-rc.2` succeeds.

```console
$ semstat type v1.2.3-preview.1
semstat: version "v1.2.3-preview.1" has an unsupported prerelease suffix "preview.1"; supported are -alpha.N, -beta.N, -rc.N, -next.N or -next.internal.N, or no suffix at all for a stable release
$ echo $?
2
```

### compare

How does `a` sort against `b`? `-1` below, `0` equal, `1` above.

```console
$ semstat compare v2.1.0 v2.0.9
1

$ semstat compare v2.0.0-rc.2 v2.0.0
-1

$ semstat compare v1.0.0+build.1 v1.0.0+build.2
0
```

Two valid versions always have an answer, so all three of those exit 0. Exit 2
means one of them could not be read, never that they compared unfavourably.

### gt

Is `a` strictly above `b`? A condition, printing nothing.

```console
$ semstat gt v2.0.0 v1.9.9   ; echo $?   # 0, yes
$ semstat gt v1.9.9 v2.0.0   ; echo $?   # 1, no
$ semstat gt v2.0.0 v2.0.0   ; echo $?   # 1, equal is not above
$ semstat gt v2.0.0 nope     ; echo $?   # 2, unreadable
```

Strictly above, so equal versions answer no. For "newer or the same", use
`compare` and accept `0` alongside `1`.

### version and help

```console
$ semstat version
1.2.3
```

The version is stamped at release time, so a binary built from source reports
`dev`. `help`, `-h` and `--help` print the usage to stdout. Running `semstat` with no
command at all prints the same text to stderr and exits 64, so a bare invocation
in a pipeline fails instead of feeding usage text downstream.

### Exit codes

Four codes, not two:

- **0** the command succeeded, or the answer is yes
- **1** the answer is no
- **2** the input could not be understood
- **64** the command was misused

Only `validate` and `gt` have an answer that means no. Every other command
reports an unreadable version as 2. The split matters, because the alternative
is the bug this tool was built to remove: without a distinct code, a typo in
`$candidate` is indistinguishable from a legitimate "not newer".

64 continues that split one step out. A caller may reasonably treat 2 from
`parse` or `type` as "not a version we can use" and carry on, and that is only
safe while 2 cannot also mean "this binary does not have that subcommand". So an
unknown command, a wrong argument count and a bad flag are 64, on every command
including `version` and `help`. A caller that accepts 2 as an answer should treat
64 as a hard failure: it is never a fact about the version, only about the call.

```bash
status=0
parsed="$(semstat parse "$tag")" || status=$?
case $status in
  0) ;;                                  # readable, use $parsed
  2) echo "$tag is not a version" ;;     # an answer, carry on
  *) echo "semstat is being called wrong" >&2
     exit 1 ;;
esac
```

Capture the status through `|| status=$?` rather than reading `$?` on the next
line, so the branch survives `set -e`, which would otherwise take the failing
assignment as the end of the script.

The numbers are borrowed rather than invented. 0, 1 and 2 are the contract `grep`
and `diff` use for yes, no and trouble. 64 is `EX_USAGE` from `sysexits.h`,
because that family has no number of its own for "you called me wrong" and
overloads 2 with it, which is the bug this split exists to remove. The gap is
deliberate: 0, 1 and 2 are all conclusions about a version, and 64 is visibly not
one.

Reading a non-zero exit takes a little care, because the shell hides it in
exactly the places you would want it. `set -e` does not apply to a condition, so
`if` and `&&` swallow every non-zero status alike, and `$(...)` discards the
status of the command inside it. So branch on the code rather than on the
command:

```bash
if semstat gt "$candidate" "$current"; then
  echo "newer"
elif [ $? -eq 1 ]; then
  echo "not newer"
else
  exit 1          # 2 or 64: not an ordering answer at all
fi
```

and for the commands that print, assign first, where the status survives:

```bash
release_type="$(semstat type "$tag")" || exit 1
case "$release_type" in
  ...
esac
```

## Recipes

Each of these is a job our release tooling actually does.

### Is this tag newer than what is released?

The incumbent comes from the published releases, not from `git tag`: a tag can
exist without a release, and a release can be a draft nobody has cut yet.

```bash
candidate="${GITHUB_REF_NAME:-$(git describe --tags --exact-match)}"
latest="$(gh release list --exclude-drafts --exclude-pre-releases \
            --limit 1 --json tagName --jq '.[0].tagName')"

if semstat gt "$candidate" "$latest"; then
  echo "$candidate supersedes $latest"
elif [ $? -eq 1 ]; then
  echo "$candidate is not newer than $latest"
else
  echo "one of $candidate or $latest is not a version" >&2
  exit 1
fi
```

On a repository whose only releases are prereleases, `$latest` comes back empty
and that third branch is the one that fires, which is the distinction the exit
codes exist for: nothing to compare against is not the same answer as "older".

### Should this tag move `latest`?

Two questions, not one. A prerelease never moves a floating tag, and a stable
tag still must not move it backwards. The pipeline here enforces the first and
not the second, so a job that promotes images should ask both.

```bash
release_type="$(semstat type "$TAG")" || exit 1
[ "$release_type" = stable ] || { echo "$release_type: leaving latest alone"; exit 0; }

if semstat gt "$TAG" "$CURRENT_LATEST"; then
  docker buildx imagetools create \
    -t ghcr.io/loft-sh/semstat:latest \
    "ghcr.io/loft-sh/semstat:${TAG#v}"
elif [ $? -eq 1 ]; then
  echo "refusing: $TAG is older than $CURRENT_LATEST" >&2
  exit 1
else
  echo "refusing: cannot compare $TAG with $CURRENT_LATEST" >&2
  exit 1
fi
```

### Pick the newest of a list

There is no `sort` subcommand on purpose: comparison was the part that was
wrong, and a loop around `gt` is the whole of it. Validate first, so `gt` only
ever sees two real versions and its answer is yes or no rather than three-way.

```bash
newest=""
while read -r tag; do
  status=0
  semstat validate "$tag" 2>/dev/null || status=$?
  case $status in
    0) ;;                                          # a version, consider it
    1) echo "skipping $tag" >&2; continue ;;       # not a version
    *) echo "cannot read the tag list" >&2         # 64: we are calling it wrong
       exit 1 ;;
  esac

  if [ -z "$newest" ] || semstat gt "$tag" "$newest"; then
    newest="$tag"
  fi
done < <(git tag -l 'v*')
echo "$newest"
```

Branch on `validate`'s code rather than using it as a bare condition. 1 means
"not a version, skip it", but 64 means we are calling semstat wrong, and skipping
on that would work through every tag and print the newest of nothing. Once
`validate` has passed, `gt` is comparing two real versions, so its answer there
really is only yes or no.

Read from a process substitution, not `git tag -l | while`: a pipeline runs the
loop in a subshell, and `$newest` is empty again by the time the loop ends.

Given `v1.9.0`, `v2.0.0-rc.2`, `v2.0.0`, `v1.10.0` and `v2.0.0+build.5`, this
picks `v2.0.0`. `sort -V | tail -1` picks `v2.0.0-rc.2`, which is the promotion
this tool was written to unblock.

### Route a tag to a channel

Assign first. `case` on an unquoted command substitution would see an empty
string when the tag is unroutable, match nothing, and continue in silence.

```bash
release_type="$(semstat type "$TAG")" || exit 1
case "$release_type" in
  stable)             publish_to stable ;;
  rc|beta|alpha)      publish_to prerelease ;;
  next|next-internal) publish_to internal ;;
esac
```

### Which release line is this tag on?

For picking a backport target, or naming a floating image tag.

```bash
# Piping straight into jq would hide a parse failure, because jq exits 0 on
# empty input and the assignment would succeed as "v".
parsed="$(semstat parse "$TAG")" || exit 1
line="$(jq -r '"v\(.major).\(.minor)"' <<<"$parsed")"   # v2.0.3 -> v2.0
```

### From a workflow step

```yaml
- name: Classify the tag
  id: release
  env:
    TAG: ${{ github.ref_name }}
  run: |
    set -euo pipefail
    # Assign, then echo. `echo "type=$(semstat type "$TAG")"` keeps the step
    # green on an unroutable tag, because echo succeeds and the substitution's
    # status is discarded, and every later `if:` then reads an empty string as
    # "not stable" and skips silently.
    release_type="$(semstat type "$TAG")"
    echo "type=$release_type" >> "$GITHUB_OUTPUT"

- name: Publish the stable artifacts
  if: steps.release.outputs.type == 'stable'
  run: ./hack/publish-stable.sh
```

`semstat validate` writes nothing on success, so it reads as a plain condition:

```bash
semstat validate "$version" || exit 1
```

## The release-type vocabulary

`semstat type` is the one thing here no off-the-shelf tool does, and it is
deliberately narrow:

| Suffix | Type |
| -- | -- |
| none | `stable` |
| `-alpha.N` | `alpha` |
| `-beta.N` | `beta` |
| `-rc.N` | `rc` |
| `-next.N` | `next` |
| `-next.internal.N` | `next-internal` |

The counter is part of the shape, not decoration. `-rc.2` is an `rc`; a bare
`-rc` is an error, and so is the undotted `-rc2`. `-next.internal` is an error
too, because it is far more often `-next.internal.N` with the counter left off
than it is a `next` counted by the word "internal", and guessing either way
silently is worse than stopping.

The identifier count is exact. `-next.N` and `-next.internal.N` are the only
accepted `next` shapes, so `-rc.1.extra` and `-next.internalX.1` are both
errors: a trailing identifier means the tag was cut to a shape nobody defined.

Anything else is an error, including suffixes that are perfectly legal semver
such as `-preview.1` or `-vendor.alpha.1`. That is on purpose: a release type
nobody knows how to route must stop the pipeline, not get silently sorted into
the closest-looking branch.

`semstat validate` takes the wider view and accepts any valid semver, because
"is this parseable" and "is this a version we route" are different questions.

## Verifying a download

Releases are built by GitHub Actions, signed keyless with cosign, and carry SLSA
build provenance.

Give the identity exactly, tag included. A regexp ending in `@.*` would also
accept a signature made by a run on some other ref, which is most of what the
signature is there to rule out.

```bash
tag=v1.2.3
identity="https://github.com/loft-sh/semstat/.github/workflows/release.yaml@refs/tags/${tag}"
```

The release assets. `checksums.txt` is what carries a signature; the archives are
covered through it, which is why the checksum check is part of verifying and not
a separate courtesy.

```bash
gh release download "$tag" --repo loft-sh/semstat
cosign verify-blob checksums.txt --bundle checksums.txt.sigstore.json \
  --certificate-identity="$identity" \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com
sha256sum --ignore-missing -c checksums.txt
```

The image, by digest. Signatures are keyed to a digest, and a tag can be moved
onto another one, so resolve it first and verify what you resolved.

```bash
digest="$(docker buildx imagetools inspect "ghcr.io/loft-sh/semstat:${tag#v}" \
            --format '{{ .Manifest.Digest }}')"

# --new-bundle-format=false is required. The image signature sits on a
# sha256-<digest>.sig tag, and while cosign 3 does fall back to that tag when it
# finds no bundle, the provenance attestation on the same digest is one, and
# cosign stops on a bundle it cannot verify rather than trying the tag.
cosign verify "ghcr.io/loft-sh/semstat@${digest}" \
  --new-bundle-format=false \
  --certificate-identity="$identity" \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com
```

The build provenance, which says which workflow and which run produced that
digest. Use `gh` rather than cosign here: the attestation is signed through
GitHub's own Sigstore instance, and cosign judges it against the public
transparency log, where it has no entry.

```bash
gh attestation verify "oci://ghcr.io/loft-sh/semstat@${digest}" \
  --repo loft-sh/semstat \
  --signer-workflow loft-sh/semstat/.github/workflows/release.yaml \
  --source-ref "refs/tags/${tag}"
```

## Development

```bash
make test       # go test ./... -race -cover
make lint       # gofmt + go vet
make build      # ./semstat
make snapshot   # full release build, publishing nothing
```

`Masterminds/semver` is the only version-handling dependency, and the intent is
that it stays the only one.

## Releasing

Push a tag; the pipeline does the rest.

```bash
git tag -s v1.2.3 && git push origin v1.2.3
```

Re-running an existing tag is a `workflow_dispatch` on `release.yaml`, because
force-pushing a tag does not re-trigger a workflow. Re-runs replace the release
artifacts and notes rather than appending to them.

A prerelease tag moves neither `latest` nor the floating `{major}.{minor}` image
tag, and is marked as a prerelease on GitHub. Releases are otherwise assumed to
be cut in ascending order: nothing checks that a tag is newer than the current
`latest`, so cutting an older patch after a newer minor would move `latest`
backwards onto it.

The Homebrew cask is pushed to `loft-sh/homebrew-tap` with `HOMEBREW_TAP_TOKEN`,
which must be a `loft-bot` token: pushing to that repo's default branch needs
approval, per the GitHub Actions Developer Guide. If the secret is absent the
cask upload is skipped and the rest of the release still publishes.
