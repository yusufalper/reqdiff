FROM golang:1.26.0-bookworm AS source
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
COPY testdata ./testdata

FROM source AS test
RUN go test -race ./... && go vet ./...

FROM source AS build
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /reqdiff ./cmd/reqdiff

FROM scratch AS runtime
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /reqdiff /reqdiff
USER 65532:65532
ENTRYPOINT ["/reqdiff"]
