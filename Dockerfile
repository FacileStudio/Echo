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
COPY head.html             /usr/share/jitsi-meet/head.html
COPY base.html             /usr/share/jitsi-meet/base.html
COPY title.html            /usr/share/jitsi-meet/title.html
COPY fonts.html            /usr/share/jitsi-meet/fonts.html
COPY index.html            /usr/share/jitsi-meet/index.html

# Also overwrite the defaults template so entrypoint generates correct values
COPY interface_config.js  /defaults/interface_config.js

# Store Echo overrides so the s6 init script can re-apply them after
# the jitsi/web entrypoint regenerates its defaults
COPY interface_config.js  /echo-overrides/interface_config.js
COPY head.html             /echo-overrides/head.html
COPY base.html             /echo-overrides/base.html
COPY title.html            /echo-overrides/title.html
COPY fonts.html            /echo-overrides/fonts.html
COPY index.html            /echo-overrides/index.html

# s6 init script that runs last (99) to re-apply Echo branding
# after jitsi/web entrypoint regenerates its template files
RUN printf '#!/bin/bash\ncp /echo-overrides/* /usr/share/jitsi-meet/\n' \
    > /etc/cont-init.d/99-echo-branding \
    && chmod +x /etc/cont-init.d/99-echo-branding
