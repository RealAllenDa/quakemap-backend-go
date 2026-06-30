FROM golang:1.26-alpine AS build
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /quakemap ./cmd/quakemap

FROM alpine:3.22
RUN adduser -D -H app
WORKDIR /app
COPY --from=build /quakemap /usr/local/bin/quakemap
COPY static ./static
COPY assets ./assets
RUN chown app:app /app
USER app
EXPOSE 9090
ENTRYPOINT ["quakemap"]
