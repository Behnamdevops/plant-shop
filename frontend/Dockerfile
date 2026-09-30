# syntax=docker/dockerfile:1
FROM node:22-alpine AS build
WORKDIR /app
ARG VITE_STORE_NAME
ARG VITE_PUBLIC_BASE_URL
ARG VITE_SUPPORT_EMAIL
ARG VITE_SUPPORT_PHONE
ARG VITE_SUPPORT_TELEGRAM
ARG VITE_SUPPORT_INSTAGRAM
ENV VITE_STORE_NAME=$VITE_STORE_NAME VITE_PUBLIC_BASE_URL=$VITE_PUBLIC_BASE_URL VITE_SUPPORT_EMAIL=$VITE_SUPPORT_EMAIL VITE_SUPPORT_PHONE=$VITE_SUPPORT_PHONE VITE_SUPPORT_TELEGRAM=$VITE_SUPPORT_TELEGRAM VITE_SUPPORT_INSTAGRAM=$VITE_SUPPORT_INSTAGRAM
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build && npm prune --omit=dev
FROM node:22-alpine AS runtime
WORKDIR /app
ENV NODE_ENV=production PORT=4173
COPY --from=build --chown=node:node /app/dist ./dist
COPY --from=build --chown=node:node /app/node_modules ./node_modules
COPY --from=build --chown=node:node /app/package.json /app/server.mjs ./
USER node
EXPOSE 4173
CMD ["node","server.mjs"]
