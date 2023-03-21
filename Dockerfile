FROM       alpine:3.17
MAINTAINER Maxim Pogozhiy <foxdalas@gmail.com>

COPY index-pattern-keeper /bin/index-pattern-keeper
ENTRYPOINT ["/bin/docker-cleaner"]
