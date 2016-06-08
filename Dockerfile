# Docker image for Drone's git-clone plugin
#
#     CGO_ENABLED=0 go build -a -tags netgo
#     docker build --rm=true -t plugins/drone-git .

FROM alpine:3.2
RUN apk add -U ca-certificates git openssh curl perl && rm -rf /var/cache/apk/*
# The following two lines required because sometimes we 
# performing "git commit" (for example, to tweak line endings in .gitattributes)
RUN git config --global user.email "drone-git@sourcegraph.com" 
RUN git config --global user.name "drone-git"
ADD drone-git /bin/
ENTRYPOINT ["/bin/drone-git"]
