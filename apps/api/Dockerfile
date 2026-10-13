# Runtime image only: no compiler, shell or package manager. The binary is
# built beforehand by scripts/build-artifacts.sh with the declared, pinned Go
# toolchain (CGO off, static), so the image build needs no network beyond the
# pinned base image.
FROM gcr.io/distroless/static-debian12:nonroot@sha256:afa5c872c891853ca7fcf1f12c3edb23f7eeef36189728842dd51042ff57f7ab
COPY bin/notes-api /notes-api
USER nonroot:nonroot
EXPOSE 3000
ENTRYPOINT ["/notes-api"]
