# ---- build stage ----
FROM golang:1.24-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# go-sqlite3 uses cgo, so CGO_ENABLED=1 (gcc is included in this image)
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o /helpdesk .

# ---- run stage ----
FROM debian:bookworm-slim
RUN useradd --create-home app && mkdir /data && chown app /data
COPY --from=build /helpdesk /usr/local/bin/helpdesk
USER app
ENV ADDR=:8080 DB_PATH=/data/helpdesk.db
VOLUME /data
EXPOSE 8080
CMD ["helpdesk"]
