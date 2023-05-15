FROM       alpine:3.18
MAINTAINER Maxim Pogozhiy <foxdalas@gmail.com>

COPY index-pattern-keeper /bin/index-pattern-keeper
ENTRYPOINT ["/bin/index-pattern-keeper"]
