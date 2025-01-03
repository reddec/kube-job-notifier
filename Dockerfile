FROM --platform=$BUILDPLATFORM alpine:3.20 AS certs
RUN apk add --no-cache ca-certificates && update-ca-certificates

FROM scratch
ENV CONFIG=/etc/notify.yaml
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY kube-job-notifier /bin/kube-job-notifier
ENTRYPOINT ["/bin/kube-job-notifier"]