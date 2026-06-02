# syntax=docker/dockerfile:1
FROM node:24-trixie-slim AS build
RUN corepack enable
WORKDIR /app
COPY package.json pnpm-lock.yaml .npmrc ./
RUN corepack install
RUN pnpm install --frozen-lockfile
COPY . .
RUN pnpm run build

# adapter-node bundles a self-contained server into build/, so the runtime
# image needs no node_modules.
FROM node:24-trixie-slim AS runtime
WORKDIR /app
ENV NODE_ENV=production
COPY --from=build /app/build ./build
EXPOSE 3000
USER node
ENTRYPOINT ["node", "build"]
