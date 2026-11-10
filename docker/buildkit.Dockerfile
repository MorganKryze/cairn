# Never built. This file holds the buildkit image the image and release jobs
# run their builds in, in a form Dependabot reads: it cannot watch an image
# named inside a workflow (dependabot-core#5541), so a pin there rotted until
# a weekly job noticed. Here a moved buildx-stable-1 becomes a pull request,
# as a moved golang tag does for the Dockerfile beside it.
FROM mirror.gcr.io/moby/buildkit:buildx-stable-1@sha256:c5f8862f52499773af85bc07695b2379ef08e8002352b0df2c4864c91d78dabe
