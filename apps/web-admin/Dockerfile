ARG NODE_BUILD_IMAGE=node:24-alpine
ARG WEB_RUNTIME_IMAGE=nginx:1.29-alpine

FROM ${NODE_BUILD_IMAGE} AS builder

WORKDIR /src
COPY package.json package-lock.json ./
COPY apps/web-admin/package.json apps/web-admin/package.json
COPY apps/desktop-client/package.json apps/desktop-client/package.json
COPY packages/shared-types/package.json packages/shared-types/package.json
RUN npm ci --workspace apps/web-admin --include-workspace-root

COPY apps/web-admin apps/web-admin
ARG VITE_API_BASE_URL=
ENV VITE_API_BASE_URL=${VITE_API_BASE_URL}
RUN npm --workspace apps/web-admin run build

FROM ${WEB_RUNTIME_IMAGE}

COPY apps/web-admin/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /src/apps/web-admin/dist /usr/share/nginx/html

EXPOSE 80
