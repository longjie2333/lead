# Go Cross Build Action

Build and package one Go target platform. Release publishing is intentionally left to the caller workflow.

This action is designed to be called from a matrix job:

```yaml
jobs:
  build:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        include:
          - goos: linux
            goarch: amd64
          - goos: linux
            goarch: arm64
          - goos: windows
            goarch: amd64

    steps:
      - uses: actions/checkout@v4

      - id: build
        uses: ./.github/actions/go-cross-build
        with:
          go-version-file: go.mod
          goos: ${{ matrix.goos }}
          goarch: ${{ matrix.goarch }}
          package: ./cmd/server
          binary-name: my-server
          output-dir: dist
          extra-files: |
            README.md
            LICENSE

      - uses: actions/upload-artifact@v4
        with:
          name: ${{ steps.build.outputs.artifact-name }}
          path: |
            ${{ steps.build.outputs.artifact-path }}
            ${{ steps.build.outputs.sha256-path }}
          if-no-files-found: error
```

Publishing can stay in a separate job:

```yaml
jobs:
  release:
    runs-on: ubuntu-latest
    needs: build
    if: startsWith(github.ref, 'refs/tags/v')
    permissions:
      contents: write

    steps:
      - uses: actions/download-artifact@v4
        with:
          path: dist
          merge-multiple: true

      - env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          gh release view "$GITHUB_REF_NAME" >/dev/null 2>&1 \
            || gh release create "$GITHUB_REF_NAME" --title "$GITHUB_REF_NAME" --generate-notes
          gh release upload "$GITHUB_REF_NAME" dist/* --clobber
```

For CGO cross builds, install the required C compiler in the caller workflow and pass `cgo-enabled: "1"` plus `cc`.

```yaml
- name: Install Linux arm64 compiler
  run: |
    sudo apt-get update
    sudo apt-get install -y --no-install-recommends gcc-aarch64-linux-gnu

- id: build
  uses: ./.github/actions/go-cross-build
  with:
    goos: linux
    goarch: arm64
    cgo-enabled: "1"
    cc: aarch64-linux-gnu-gcc
```

By default, the action won't runs `gofmt -l .` and `go test ./...`.