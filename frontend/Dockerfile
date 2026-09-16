# syntax=docker/dockerfile:1

# ---- Build stage -----------------------------------------------------------
FROM node:22-alpine AS build

WORKDIR /app

# Store config is baked in at build time via Vite env vars (VITE_STORE_NAME,
# VITE_CURRENCY_SYMBOL). Pass them as build args from docker compose if you
# want to customize the running store's branding.
ARG VITE_STORE_NAME
ARG VITE_CURRENCY_SYMBOL
ENV VITE_STORE_NAME=${VITE_STORE_NAME}
ENV VITE_CURRENCY_SYMBOL=${VITE_CURRENCY_SYMBOL}

COPY package.json package-lock.json ./
RUN npm ci

COPY . .
RUN npm run build

# ---- Runtime stage ----------------------------------------------------------
# nginx serves the built static assets and reverse-proxies /api/* to the
# backend service (by Docker service name), so the browser keeps using
# relative /api URLs and same-origin cookies work without CORS.
FROM nginx:1.27-alpine AS runtime

COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html

EXPOSE 80
