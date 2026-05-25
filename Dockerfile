FROM node:20-bookworm AS build

WORKDIR /src
COPY package.json package-lock.json ./
RUN npm ci --legacy-peer-deps
COPY . .
RUN make all

FROM nginx:alpine

RUN rm -rf /etc/nginx/conf.d/default.conf /usr/share/nginx/html

COPY docker/nginx.conf /etc/nginx/nginx.conf
COPY docker/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# App root
WORKDIR /srv/echo

# Built assets
COPY --from=build /src/libs/        libs/
COPY --from=build /src/css/all.css  css/all.css

# Source assets
COPY lang/      lang/
COPY images/    images/
COPY static/    static/
COPY sounds/    sounds/

# HTML + config
COPY index.html .
COPY config.js  config.js.template
COPY interface_config.js .
COPY head.html .
COPY base.html .
COPY title.html .
COPY fonts.html .
COPY body.html .
COPY plugin.head.html .
COPY manifest.json .
COPY pwa-worker.js .

EXPOSE 80

ENTRYPOINT ["/entrypoint.sh"]
