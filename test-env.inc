# vim: set filetype=make :

# Put personal options into test-env-local.inc
include $(wildcard test-env-local.inc)

export YTSAURUS_JOB_IMAGE ?= mirror.gcr.io/library/python:3.12-slim

# https://ytsaurus.tech/docs/en/admin-guide/releases

## Ytsaurus registry
YTSAURUS_REGISTRY ?= ghcr.io/ytsaurus
export YTSAURUS_REGISTRY

# options for e2e tests:
# epoch := <version>, NIGHTLY, LATEST, PREVIOUS, LEGACY, MEDIEVAL, ANCIENT
# version := ${<name>_VERSION_<epoch>} or <epoch>
# image := ${<name>_IMAGE_<version>} or ${<name>_IMAGE}:<version> or ${YTSAURUS_REGISTRY}/<name>:<version>

# https://github.com/ytsaurus/ytsaurus/pkgs/container/ytsaurus
# https://github.com/ytsaurus/ytsaurus/pkgs/container/ytsaurus-nightly
# skopeo inspect --format '{{range .RepoTags}}{{.}}\n{{end -}}' docker://ghcr.io/ytsaurus/ytsaurus-nightly

## Current "CURR" ytsaurus version or epoch for tests: X.Y[.Z], NIGHTLY, LATEST, PREVIOUS, LEGACY, MEDIEVAL, ANCIENT
override TEST_YTSAURUS_VERSION := $(or ${TEST_YTSAURUS_VERSION},PREVIOUS)
export TEST_YTSAURUS_VERSION

export YTSAURUS_IMAGE ?= ${YTSAURUS_REGISTRY}/ytsaurus

export YTSAURUS_IMAGE_26_3_99 ?= ${YTSAURUS_IMAGE}-nightly:dev-2026-10-09-af133131cb97646244d6be9e9a608c7ff5b6ed6f
export YTSAURUS_IMAGE_26_2_99 ?= ${YTSAURUS_IMAGE}-nightly:dev-26.2-2026-10-08-c4f329f8e0073e869b29ce8c1043e87ebad78e57
export YTSAURUS_IMAGE_26_1_99 ?= ${YTSAURUS_IMAGE}-nightly:dev-26.1-2026-10-07-4ff7086c5c13ac2f6ee64a9be6f1710da76f3a1e
export YTSAURUS_IMAGE_25_4_99 ?= ${YTSAURUS_IMAGE}-nightly:dev-25.4-2026-10-06-98ad930ca037547fc866e3decdde7d98673be695
export YTSAURUS_IMAGE_25_3_99 ?= ${YTSAURUS_IMAGE}-nightly:dev-25.3-2026-07-29-151a441ed08dbab506859d15fa48179ea7e98985
export YTSAURUS_IMAGE_25_2_99 ?= ${YTSAURUS_IMAGE}-nightly:dev-25.2-2026-05-07-19431d7dc6ea4bb963ae1e933c4063ffedf54db9
export YTSAURUS_IMAGE_25_1_99 ?= ${YTSAURUS_IMAGE}-nightly:dev-25.1-2025-09-30-da1f6800979c77f5a19f44dc18d970bb5a70ac03
export YTSAURUS_IMAGE_24_2_99 ?= ${YTSAURUS_IMAGE}-nightly:dev-24.2-2025-09-05-522db86a58d4950019121180a35c9b56d0d8af76

## ytsaurus latest+1 version
YTSAURUS_VERSION_NIGHTLY ?= # 26.1.99 is broken
export YTSAURUS_VERSION_NIGHTLY

## ytsaurus latest version
YTSAURUS_VERSION_LATEST ?= # 25.4.1 is broken
export YTSAURUS_VERSION_LATEST

## ytsaurus latest-1 version
YTSAURUS_VERSION_PREVIOUS ?= 25.3.1
export YTSAURUS_VERSION_PREVIOUS

## ytsaurus latest-2 version
YTSAURUS_VERSION_LEGACY ?= 25.2.2
export YTSAURUS_VERSION_LEGACY

## ytsaurus latest-3 version
YTSAURUS_VERSION_MEDIEVAL ?= 25.1.0
export YTSAURUS_VERSION_MEDIEVAL

## ytsaurus latest-4 version
YTSAURUS_VERSION_ANCIENT ?= 24.2.1
export YTSAURUS_VERSION_ANCIENT

# https://github.com/orgs/ytsaurus/packages/container/package/sidecars
# https://ytsaurus.tech/docs/en/admin-guide/install-ytmsvc

export SIDECARS_IMAGE ?= ${YTSAURUS_REGISTRY}/sidecars

export SIDECARS_VERSION_LATEST ?= 0.0.2
export SIDECARS_VERSION_PREVIOUS ?= 0.0.2
export SIDECARS_VERSION_LEGACY ?= 0.0.2
export SIDECARS_VERSION_MEDIEVAL ?= 0.0.2
export SIDECARS_VERSION_ANCIENT ?= 0.0.2

# https://github.com/ytsaurus/ytsaurus/pkgs/container/query-tracker
# https://github.com/ytsaurus/ytsaurus/pkgs/container/query-tracker-nightly
# skopeo inspect --format '{{range .RepoTags}}{{.}}\n{{end -}}' docker://ghcr.io/ytsaurus/query-tracker-nightly:dev-2025-07-23-be855609d82ae5c6952a6b5c8c4e1706d1b18232

export QUERY_TRACKER_IMAGE ?= ${YTSAURUS_REGISTRY}/query-tracker

export QUERY_TRACKER_VERSION_LATEST ?= 0.4.1
export QUERY_TRACKER_VERSION_PREVIOUS ?= 0.1.2
export QUERY_TRACKER_VERSION_LEGACY ?= 0.1.2
export QUERY_TRACKER_VERSION_MEDIEVAL ?= 0.1.2
export QUERY_TRACKER_VERSION_ANCIENT ?= 0.1.2

export QUERY_TRACKER_IMAGE_NIGHTLY ?= ${QUERY_TRACKER_IMAGE}-nightly:dev-2026-10-09-af133131cb97646244d6be9e9a608c7ff5b6ed6f

export QUERY_TRACKER_IMAGE_0_4_1 ?= ${QUERY_TRACKER_IMAGE}:stable-0.4.1

# https://github.com/ytsaurus/ytsaurus/pkgs/container/strawberry
# https://github.com/ytsaurus/ytsaurus/pkgs/container/strawberry-nightly
# skopeo inspect --format '{{range .RepoTags}}{{.}}\n{{end -}}' docker://ghcr.io/ytsaurus/strawberry-nightly:dev-2025-07-23-8c5b9c9b908af8b2f883de051b62072e795d40d3

export STRAWBERRY_IMAGE ?= ${YTSAURUS_REGISTRY}/strawberry

export STRAWBERRY_VERSION_LATEST ?= 0.0.18
export STRAWBERRY_VERSION_PREVIOUS ?= 0.0.18
export STRAWBERRY_VERSION_LEGACY ?= 0.0.18
export STRAWBERRY_VERSION_MEDIEVAL ?= 0.0.18
export STRAWBERRY_VERSION_ANCIENT ?= 0.0.18

export STRAWBERRY_IMAGE_NIGHTLY ?= ${STRAWBERRY_IMAGE}-nightly:dev-2026-10-14-c9931c9ef455e646c90bb3b188754cd4fa3df5c4

# https://github.com/ytsaurus/ytsaurus/pkgs/container/chyt
# https://github.com/ytsaurus/ytsaurus/pkgs/container/chyt-nightly
# skopeo inspect --format '{{range .RepoTags}}{{.}}\n{{end -}}' docker://ghcr.io/ytsaurus/chyt-nightly:dev-2025-07-23-85732937466aec6368c1582345afdedff70c5007

export CHYT_IMAGE ?= ${YTSAURUS_REGISTRY}/chyt

export CHYT_VERSION_LATEST ?= 2.19.0
export CHYT_VERSION_PREVIOUS ?= 2.18.6
export CHYT_VERSION_LEGACY ?= 2.18.6
export CHYT_VERSION_MEDIEVAL ?= 2.18.6
export CHYT_VERSION_ANCIENT ?= 2.18.6

export CHYT_IMAGE_NIGHTLY ?= ${CHYT_IMAGE}-nightly:dev-2026-10-09-af133131cb97646244d6be9e9a608c7ff5b6ed6f

define LOAD_TEST_IMAGES
$(YTSAURUS_IMAGE_$(subst .,_,$(YTSAURUS_VERSION_LATEST)))
$(YTSAURUS_IMAGE_$(subst .,_,$(YTSAURUS_VERSION_PREVIOUS)))
$(YTSAURUS_IMAGE_$(subst .,_,$(YTSAURUS_VERSION_LEGACY)))
${QUERY_TRACKER_IMAGE}:${QUERY_TRACKER_VERSION_CURRENT}
${CHYT_IMAGE}:${CHYT_VERSION_CURRENT}
endef

SAMPLE_YTSAURUS_VERSION ?= ${TEST_YTSAURUS_VERSION}
SAMPLE_YTSAURUS_FLAVOR ?= -relwithdebinfo

YTSAURUS_SAMPLE_IMAGE ?= ${YTSAURUS_IMAGE}:stable-${YTSAURUS_VERSION_${SAMPLE_YTSAURUS_VERSION}}${SAMPLE_YTSAURUS_FLAVOR}

QUERY_TRACKER_SAMPLE_IMAGE ?= ${QUERY_TRACKER_IMAGE}:${QUERY_TRACKER_VERSION_${SAMPLE_YTSAURUS_VERSION}}
STRAWBERRY_SAMPLE_IMAGE ?= ${STRAWBERRY_IMAGE}:${STRAWBERRY_VERSION_${SAMPLE_YTSAURUS_VERSION}}
CHYT_SAMPLE_IMAGE ?= ${CHYT_IMAGE}:${CHYT_VERSION_${SAMPLE_YTSAURUS_VERSION}}

define LOAD_SAMPLE_IMAGES
${YTSAURUS_SAMPLE_IMAGE}
${QUERY_TRACKER_SAMPLE_IMAGE}
${STRAWBERRY_SAMPLE_IMAGE}
${CHYT_SAMPLE_IMAGE}
endef
