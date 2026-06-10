# ---------- build stage ----------
FROM golang:1.21-bookworm AS build
WORKDIR /src
ENV GOFLAGS=-mod=mod
COPY go.mod ./
COPY . .
# Stdlib-only service: builds fully offline, no module downloads required.
RUN CGO_ENABLED=0 go build -o /out/go-api .

# ---------- runtime stage ----------
FROM gcr.io/distroless/static-debian12
COPY --from=build /out/go-api /usr/local/bin/go-api
EXPOSE 8090
ENTRYPOINT ["go-api"]
