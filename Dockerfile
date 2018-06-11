FROM golang:latest

MAINTAINER Anshuman Bhartiya

RUN go get github.com/anshumanbh/waybackurls

ENTRYPOINT [ "waybackurls" ]
