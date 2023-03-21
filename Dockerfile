FROM       alpine:3.16
MAINTAINER Maxim Pogozhiy <foxdalas@gmail.com>

COPY index-pattern-keeper /bin/index-pattern-keeper
ENTRYPOINT ["/bin/docker-cleaner"]
