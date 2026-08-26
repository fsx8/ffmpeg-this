# ffwiz container image — used by goreleaser dockers_v2.
# goreleaser copies the prebuilt static binaries into the build context under
# $TARGETPLATFORM/, so this Dockerfile only provides the runtime (ffmpeg).
FROM alpine:3

RUN apk add --no-cache ffmpeg ca-certificates tzdata \
    && addgroup -S ffwiz \
    && adduser -S -G ffwiz -h /media ffwiz

ARG TARGETPLATFORM
COPY $TARGETPLATFORM/ffwiz /usr/local/bin/ffwiz
RUN chmod +x /usr/local/bin/ffwiz

WORKDIR /media
USER ffwiz
ENTRYPOINT ["ffwiz"]
