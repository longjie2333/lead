# Android Build Action

Build one Android variant and collect APK/AAB outputs. Release publishing is intentionally left to the caller workflow.

The action is designed to be called once per variant. If a project has multiple flavors or build types, fan them out in the caller workflow with `strategy.matrix`.

```yaml
jobs:
  android:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        include:
          - variant: release
            tasks: |
              testReleaseUnitTest
              lintRelease
              assembleRelease
              bundleRelease

    steps:
      - uses: actions/checkout@v4

      - id: build
        uses: ./.github/actions/android-build
        with:
          working-directory: app
          gradle-tasks: ${{ matrix.tasks }}
          version-code: ${{ github.run_number }}
          version-name: ${{ github.ref_name }}
          artifact-name: example-android-${{ matrix.variant }}
          project-properties: |
            apiBaseUrl=${{ secrets.API_BASE_URL }}
          signing-keystore-base64: ${{ secrets.ANDROID_KEYSTORE_BASE64 }}
          signing-store-password: ${{ secrets.ANDROID_KEYSTORE_PASSWORD }}
          signing-key-alias: ${{ secrets.ANDROID_KEY_ALIAS }}
          signing-key-password: ${{ secrets.ANDROID_KEY_PASSWORD }}

      - uses: actions/upload-artifact@v4
        with:
          name: ${{ steps.build.outputs.artifact-name }}
          path: ${{ steps.build.outputs.artifact-dir }}/*
          if-no-files-found: error
```

Publishing can stay in a separate job:

```yaml
jobs:
  release:
    runs-on: ubuntu-latest
    needs: android
    if: startsWith(github.ref, 'refs/tags/v')
    permissions:
      contents: write

    steps:
      - uses: actions/download-artifact@v4
        with:
          path: dist/android
          merge-multiple: true

      - env:
          GH_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        run: |
          mapfile -d '' files < <(find dist/android -type f -print0)
          if [ "${#files[@]}" -eq 0 ]; then
            echo "No Android artifacts found."
            exit 1
          fi

          gh release view "$GITHUB_REF_NAME" >/dev/null 2>&1 \
            || gh release create "$GITHUB_REF_NAME" --title "$GITHUB_REF_NAME" --generate-notes
          gh release upload "$GITHUB_REF_NAME" "${files[@]}" --clobber
```

Important inputs:

- `working-directory`: Gradle project root.
- `gradle-tasks`: tasks to run, such as `testReleaseUnitTest`, `lintRelease`, `assembleRelease`, and `bundleRelease`.
- `project-properties`: newline-separated `key=value` values that become Gradle `-P` properties.
- `artifact-name`: base name for copied APK/AAB files and the collected artifact directory.
- `apk-globs` / `aab-globs`: override these when a project writes outputs outside the standard Android Gradle Plugin directories.
