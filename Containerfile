FROM docker.io/alpine:3.22.1

ENV CONFIG_FILE="/config/config.yml"
ENV MAIL_TEMPLATE_FILE="/config/mail.html"

WORKDIR /

EXPOSE 8080

USER 1000:1000

ADD --chown=1000:1000 --chmod=755 ./dist/reqq /usr/bin/reqq
ADD --chown=1000:1000 --chmod=755 ./conf/ /config/

ENTRYPOINT /usr/bin/reqq
