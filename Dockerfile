FROM scratch
COPY vm /bin/vm
ENTRYPOINT ["/bin/vm"]
