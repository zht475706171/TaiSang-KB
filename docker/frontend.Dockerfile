# Build stage
FROM node:20-alpine AS builder

WORKDIR /src
COPY frontend/package*.json ./
RUN npm ci

COPY frontend/ .
RUN npm run build

# Runtime stage
FROM nginx:1.27-alpine

COPY --from=builder /src/dist /usr/share/nginx/html
COPY docker/frontend.nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80