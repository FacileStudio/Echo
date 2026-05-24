ARG JITSI_IMAGE_VERSION=stable

FROM node:20-bookworm AS build

WORKDIR /src
COPY package.json package-lock.json ./
RUN npm ci --legacy-peer-deps
COPY . .
RUN make all

FROM jitsi/web:${JITSI_IMAGE_VERSION}

RUN rm -rf /usr/share/jitsi-meet/libs \
           /usr/share/jitsi-meet/css \
           /usr/share/jitsi-meet/lang \
           /usr/share/jitsi-meet/images \
           /usr/share/jitsi-meet/static \
           /usr/share/jitsi-meet/sounds

COPY --from=build /src/libs/          /usr/share/jitsi-meet/libs/
COPY --from=build /src/css/all.css    /usr/share/jitsi-meet/css/all.css
COPY --from=build /src/lang/          /usr/share/jitsi-meet/lang/
COPY --from=build /src/images/        /usr/share/jitsi-meet/images/
COPY --from=build /src/static/        /usr/share/jitsi-meet/static/
COPY --from=build /src/sounds/        /usr/share/jitsi-meet/sounds/

COPY interface_config.js  /usr/share/jitsi-meet/interface_config.js
COPY title.html            /usr/share/jitsi-meet/title.html
COPY fonts.html            /usr/share/jitsi-meet/fonts.html
COPY index.html            /usr/share/jitsi-meet/index.html
