# GRPC over IAP

This is a small library that allows you to run GRPC API's behind [Google's IAP](https://cloud.google.com/security/products/iap?hl=en). This is helpful to secure API's without having to handle complex auth yourself.

i.e. if you run admin tools from something like a laptop or GCE instance, IAP is a good way to make that admin tool exposable but still secure.

If you use this star it on github! I'll be motivated to maintain this externally if I know people use it.


> [!WARNING]  
> Currently unstable - I wouldn't use this package until this warning is removed and the API is stabilized. This depends on an unreleased GCP feature at the moment.
>
> Also, this makes a lot of assumptions about how you run your service; i.e. you're running behind port 443. I'm happy to accept PR's to remove these assumptions.

## Why is this hard?

Many reasons.
- **GRPC wants to communicate over HTTP2**. This is very tricky to do behind a load balancer - most, like GCP LB and AWS ELB choose to not support it at all, and just tell all http to be done over 1.1.
- **GRPC wants to do its own TLS**. This is a non-starter: IAP needs to decode request headers to make sure the identity token is correct
.
- **IAP has second class support for programmatic access**. Minting a "proper" OAuth2 token is more complex than trivial.

## Setup / Prereqs

In `examples/setup.yaml`, there are example manifests on how to set this up on GKE.

You will need a service account with `iap.httpsResourceAccessor` role. The machine issuing the request will also need permissions to mint tokens on their behalf.

## Running Locally

When running locally, follow the [docs here](https://github.com/stackrox/go-grpc-http1) directly.

