ARG KRAKEND_VERSION=2.1.2
ARG BUILDER_IMG=devopsfaith/krakend-plugin-builder
ARG RUN_IMG=devopsfaith/krakend

FROM ${BUILDER_IMG}:${KRAKEND_VERSION} as plugin-builder

COPY . /app/
WORKDIR /app
RUN go build -buildmode=plugin -o identityauth.so ./identityauth

FROM ${RUN_IMG}:${KRAKEND_VERSION}
COPY --from=plugin-builder /app/identityauth.so /opt/infratographer/modules/

COPY config/ /etc/krakend
