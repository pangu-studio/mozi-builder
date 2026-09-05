# The PoC build script compiles this pure-Go binary for the Docker server's
# architecture before assembling the image. No source/dependency download in Docker.
FROM scratch
COPY bin/mozi-poc /poc
USER 10001
ENTRYPOINT ["/poc"]
