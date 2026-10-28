FROM node:22-alpine AS frontend-build
WORKDIR /src
RUN corepack enable && corepack prepare pnpm@11.25.0 --activate
COPY package.json pnpm-workspace.yaml pnpm-lock.yaml ./
COPY frontend/package.json frontend/package.json
RUN pnpm install --frozen-lockfile
COPY frontend frontend
RUN pnpm --filter frontend build

FROM golang:1.26-alpine AS backend-build
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/livepoll ./cmd/server

FROM alpine:3.21 AS runtime
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=backend-build /out/livepoll /app/livepoll
COPY --from=frontend-build /src/frontend/dist /app/frontend/dist
ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["/app/livepoll"]
