# ---------- build stage ----------
FROM golang:1.21-bookworm AS build
WORKDIR /src
# -mod=mod lets the build resolve and record dependencies (go.sum) on the fly,
# so a pre-existing go.sum is not required to build the image.
ENV GOFLAGS=-mod=mod
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 go build -o /out/elt . \
 && CGO_ENABLED=0 go build -o /out/elt-advanced ./cmd/advanced

# ---------- runtime stage ----------
FROM debian:bookworm-slim
WORKDIR /app
COPY --from=build /out/elt /usr/local/bin/elt
COPY --from=build /out/elt-advanced /usr/local/bin/elt-advanced
COPY config ./config
RUN mkdir -p /app/state
ENTRYPOINT ["elt", "--config", "config/tenants.json", "--watermark", "/app/state/watermark.json"]
