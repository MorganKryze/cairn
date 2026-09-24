# Static hosting

`cairn -export` writes the whole site as files: the pages in every language,
cairn's own assets, your `assets/`, `media/` and `fonts/`, and the rules a
static host needs to send the headers cairn sends. Upload them to Cloudflare
Pages, Netlify or an Apache host such as OVH shared hosting, and no cairn
process runs anywhere.

## What changes without a server

A static host answers with files and runs nothing of cairn's, so three things
behave differently.

- **No status pills.** cairn polls your monitor from the server, and a file
  cannot. The export drops the pills, their script and the `connect-src` they
  needed, and says so:

  ```console
  export: status pills need a server to poll https://status.example.org; the exported pages carry none
  ```

- **The browser picks the language.** The server answers `/` with a redirect
  to the visitor's language. The export writes a root page that makes the same
  choice in the browser: the language they picked from the switcher, then the
  browser's own. With JavaScript off, a visitor lands on your first locale.
- **An edit needs a new export.** The server picks up a YAML change within
  seconds; the files keep the config you exported. `security.txt` expires one
  year after the export, so export at least once a year if you set
  `security.contact`.

## Export

`site.url` is required: the sitemap, `robots.txt` and the canonical links need
the address the files will be served at.

With the binary from a [release](https://github.com/MorganKryze/cairn/releases),
here for Linux on amd64 (`linux_arm64` and `darwin_arm64` are there too):

```sh
curl -fsSL https://github.com/MorganKryze/cairn/releases/download/v1.24.0/cairn_1.24.0_linux_amd64.tar.gz | tar -xz cairn
./cairn -config ./config -assets ./assets -export site.zip
```

With the image, and nothing else installed:

```sh
docker run --rm --user "$(id -u):$(id -g)" -v "$PWD:/work" \
  morgankryze/cairn:1.24.0 -config /work/config -assets /work/assets -export /work/site.zip
```

Keep `--user`. The image runs as `nobody`, and `nobody` cannot write into a
directory you own.

The name you give picks the shape:

| `-export` ends in           | You get                                   |
| --------------------------- | ----------------------------------------- |
| `.zip`                      | a zip archive                             |
| `.tar`, `.tar.gz` or `.tgz` | a tar archive, gzipped for the last two   |
| anything else               | a directory, which has to be new or empty |

cairn builds the output under a temporary name beside it and moves it into
place once every file is written, so a failed export leaves the previous one
where it stood. Two exports of the same config on the same day are the same
bytes, and a pipeline can skip an upload that changes nothing.

A file or folder whose name starts with a dot stays out, as the server refuses
to serve one: point `-assets` at a git checkout and its `.git/` does not reach
the archive.

To serve under a sub-path, pass the `-base-path` you would give the server.
The export then belongs in a folder of that name: with `-base-path /tools`, on
Apache, unpack it into `www/tools/`. On Cloudflare Pages and Netlify, deploy a
folder that holds it as `tools/`, and move `_headers` and `_redirects` up
beside `tools/`: those hosts read them at the root only.

## What is in it

```text
index.html               the root page that picks a language
404.html                 the page for an address that leads nowhere, in your first locale
en/index.html            each home, detail page and hosted page, per language
en/404.html              the 404 page in that language
static/                  cairn's stylesheet, scripts, icons and font
assets/ media/ fonts/    your files
robots.txt  sitemap.xml  manifest.webmanifest  custom.css  .well-known/security.txt
_headers  _redirects     the rules for Cloudflare Pages and Netlify
.htaccess                the same rules for Apache
```

`custom.css` and `security.txt` appear when your config has them, and
`_redirects` when `favicon.ico` has somewhere to redirect to.

The rules carry the headers the server puts on every response: the
Content-Security-Policy, HSTS when `site.url` is https, and the six other
hardening headers. They cache cairn's stamped assets for a year, send each
language its own 404 page, and point `/favicon.ico` at your icon when you set
one. Each host reads its own file and ignores the other.

## Cloudflare Pages

Create a project with Direct Upload and drop in the archive, or deploy the
directory from a pipeline:

```sh
npx wrangler pages deploy site --project-name tools
```

Pages reads `_headers` and `_redirects`, and answers a missing address with
the nearest `404.html` up the tree, so `/fr/typo` gets the French page.

## Netlify

Drag the directory or the archive onto the deploy screen, or run
`netlify deploy --prod --dir site`. Netlify reads the same two files and
answers every missing address with the root `404.html`, in your first locale.

## Apache and OVH shared hosting

Upload the contents of the directory into the web root, `www/` on OVH, over
FTP or SFTP. `.htaccess` does the rest. It needs `AllowOverride` to let it
set error pages and headers, which shared hosting allows, and `mod_headers`
for the headers themselves. Without `mod_headers` the site works and goes out
without them.

## GitHub Pages and the rest

Any host that serves files serves the site. GitHub Pages answers a missing
address with the root `404.html`, and reads neither rules file, so the pages
go out without cairn's headers.

## From a pipeline

Your config and assets live in a repository; each push exports the site and
ships it. This GitHub Actions workflow exports with the image and keeps the
result as a build artifact:

```yaml
name: site
on:
  push:
    branches: [main]

jobs:
  export:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: export the site
        run: |
          docker run --rm --user "$(id -u):$(id -g)" -v "$PWD:/work" \
            morgankryze/cairn:1.24.0 -config /work/config -assets /work/assets -export /work/site
      - uses: actions/upload-artifact@v4
        with:
          name: site
          path: site
          include-hidden-files: true
```

`include-hidden-files` keeps `.htaccess` and `.well-known/` in the artifact.
The image stays pinned to one version, so a release does not change your site
between two pushes: move the tag once you have tried the new one.

The deploy steps below follow each host's own documentation, and have not yet
run end to end in this project's CI. If one fails for you,
[open an issue](https://github.com/MorganKryze/cairn/issues) with the step's
log.

Then add the step for your host. For Cloudflare Pages, with an API token and
the account id as repository secrets:

```yaml
      - name: deploy to Cloudflare Pages
        env:
          CLOUDFLARE_API_TOKEN: ${{ secrets.CLOUDFLARE_API_TOKEN }}
          CLOUDFLARE_ACCOUNT_ID: ${{ secrets.CLOUDFLARE_ACCOUNT_ID }}
        run: npx wrangler pages deploy site --project-name tools
```

For OVH or any host you reach over SFTP, with `lftp`:

```yaml
      - name: deploy over SFTP
        env:
          SFTP_USER: ${{ secrets.SFTP_USER }}
          SFTP_PASSWORD: ${{ secrets.SFTP_PASSWORD }}
        run: |
          sudo apt-get update && sudo apt-get install -y lftp
          lftp -u "$SFTP_USER,$SFTP_PASSWORD" sftp://ssh.example.net \
            -e "mirror --reverse --delete site www; quit"
```

`--delete` removes from `www/` whatever the new export no longer has. Leave it
out if the folder holds anything cairn did not write.

Next: [Reverse proxies](reverse-proxies.md)
