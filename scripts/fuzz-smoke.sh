#!/usr/bin/env sh

set -eu

for target in \
	FuzzRequestSpecURL \
	FuzzHeaderValidation \
	FuzzAuthenticationInputs \
	FuzzAuthenticationChallengeHeaders \
	FuzzErrorPayloadClassification \
	FuzzRedirectCredentialBoundary \
	FuzzRetryPolicy
do
	go test -run '^$' -fuzz="^${target}$" -fuzztime=1s .
done
