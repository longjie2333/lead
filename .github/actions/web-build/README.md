# Web Build Action

Builds the Vue 3 web client and verifies that the server static output exists.

## Usage

```yaml
- name: Build web assets
  uses: ./.github/actions/web-build
```

By default this action:

- installs dependencies in `server-web` with `npm ci`
- runs `npm run build`
- expects `server/public/index.html` to exist after the build

## Inputs

| Input | Default | Description |
| --- | --- | --- |
| `working-directory` | `server-web` | Web project directory, relative to the repository root. |
| `node-version` | `20` | Node.js version passed to `actions/setup-node`. |
| `cache-dependency-path` | `server-web/package-lock.json` | Lock file path used for npm cache. |
| `build-command` | `npm run build` | Build command executed in `working-directory`. |
| `output-dir` | `server/public` | Expected generated static asset directory. |

## Outputs

| Output | Description |
| --- | --- |
| `output-dir` | Absolute path to the generated static asset directory. |
