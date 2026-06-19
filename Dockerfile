# 1) Build del frontend Svelte -> backend/dist
FROM node:20-alpine AS front
WORKDIR /app
COPY frontend/ ./frontend/
RUN cd frontend && npm install && npm run build
# vite escribe en /app/backend/dist (outDir relativo)

# 2) Build del binario Go (con el dist ya generado embebido)
FROM golang:1.25-alpine AS back
WORKDIR /app/backend
COPY backend/ ./
COPY --from=front /app/backend/dist ./dist
RUN go mod tidy && CGO_ENABLED=0 go build -o /vmvchat .

# 3) Imagen final mínima
FROM alpine:3.20
WORKDIR /data
COPY --from=back /vmvchat /vmvchat
EXPOSE 8080
ENTRYPOINT ["/vmvchat"]
