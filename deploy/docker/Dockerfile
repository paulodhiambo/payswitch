FROM node:20-alpine AS frontend-build
WORKDIR /src/portal
COPY portal/package.json portal/package-lock.json ./
RUN npm ci
COPY portal ./
RUN npm run build

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG SERVICE=gateway
RUN CGO_ENABLED=0 go build -o /out/service ./cmd/${SERVICE}

FROM alpine:3.19
# ca-certificates is needed for outbound TLS (OTLP, Authentik REST API, etc.)
RUN apk --no-cache add ca-certificates
COPY --from=build /out/service /service
COPY --from=frontend-build /src/portal/dist /portal/dist
ENTRYPOINT ["/service"]
