package config

// The getting-started page: what cairn serves instead of dying when the
// config directory is empty or invalid at boot. It goes through the normal
// rendering pipeline, as an in-memory config whose welcome note carries the
// instructions.

const StarterAbout = `## Almost there

cairn is running, but it has no valid configuration yet. It watches its
config folder and picks your files up the moment they appear, no restart
needed.

- Create a file named ` + "`services.yaml`" + ` in the folder mounted at ` + "`/config`" + `
- Give each service an ` + "`id`" + `, a ` + "`url`" + ` and a ` + "`name`" + `
- ` + "`cairn -init`" + ` prints a commented starter file to copy

If the folder is not empty, one of its files is probably invalid: the
container log names the file, the line and the expected shape.

The five-minute path lives in
[Getting started](https://github.com/MorganKryze/cairn/blob/main/docs/getting-started.md).`

const StarterServices = `# yaml-language-server: $schema=https://raw.githubusercontent.com/MorganKryze/cairn/main/schema/services.json
# One entry per card. id, url and name are required, the rest is optional.
# Docs: https://github.com/MorganKryze/cairn/blob/main/docs

- id: photos
  url: https://photos.example.org
  icon: immich # a dashboard-icons slug, a URL, or an /assets path
  name: { fr: Photothèque, en: Photo library }
  desc: { fr: Sauvegarder et retrouver vos photos., en: Back up and browse your photos. }
  # category: photos    # names and order come from categories.yaml
  # tags: [images, backup, sauvegarde]

- id: files
  url: https://files.example.org
  icon: nextcloud
  name: Files
  desc: Store and share documents.
`
