# vim: set filetype=make :

export YTSAURUS_JOB_IMAGE ?= mirror.gcr.io/library/python:3.12-slim

# https://ytsaurus.tech/docs/en/admin-guide/releases

## Ytsaurus registry
YTSAURUS_REGISTRY ?= ghcr.io/ytsaurus
export YTSAURUS_REGISTRY

# options for e2e tests:
# epoch := PAST, PREVIOUS, CURRENT, COMING, FUTURE, NIGHTLY, <version>
# version := ${<name>_VERSION_<epoch>} or <epoch>
# image := ${<name>_IMAGE_<version>} or ${<name>_IMAGE}:<version> or ${YTSAURUS_REGISTRY}/<name>:<version>

# https://github.com/ytsaurus/ytsaurus/pkgs/container/ytsaurus
# https://github.com/ytsaurus/ytsaurus/pkgs/container/ytsaurus-nightly
# skopeo inspect --format '{{range .RepoTags}}{{.}}\n{{end -}}' docker://ghcr.io/ytsaurus/ytsaurus-nightly

## ytsaurus version or epoch for tests: X.Y[.Z], PAST, PREVIOUS, CURRENT, COMING, FUTURE, NIGHTLY
TEST_YTSAURUS_VERSION ?= CURRENT
export TEST_YTSAURUS_VERSION

export YTSAURUS_IMAGE ?= ${YTSAURUS_REGISTRY}/ytsaurus

## ytsaurus past version
YTSAURUS_VERSION_PAST ?= 23.2.0
export YTSAURUS_VERSION_PAST

## ytsaurus previous version
YTSAURUS_VERSION_PREVIOUS ?= 24.1.0
export YTSAURUS_VERSION_PREVIOUS

## ytsaurus current version
YTSAURUS_VERSION_CURRENT ?= 24.2.1
export YTSAURUS_VERSION_CURRENT

## ytsaurus coming version
YTSAURUS_VERSION_COMING ?= 25.1.0
export YTSAURUS_VERSION_COMING

## ytsaurus future version
YTSAURUS_VERSION_FUTURE ?= 25.2.2
export YTSAURUS_VERSION_FUTURE

## ytsaurus nightly version
YTSAURUS_VERSION_NIGHTLY ?= 26.1.99
export YTSAURUS_VERSION_NIGHTLY

export YTSAURUS_IMAGE_23_2_99 ?= ${YTSAURUS_IMAGE}-nightly:dev-23.2-2025-02-19-7dae008dd5171a9c89857f2034746ac3119842db
export YTSAURUS_IMAGE_24_1_99 ?= ${YTSAURUS_IMAGE}-nightly:dev-24.1-2025-06-18-dd4bd8bdc4d28993e7185b820930761540b29ae9
export YTSAURUS_IMAGE_24_2_99 ?= ${YTSAURUS_IMAGE}-nightly:dev-24.2-2025-09-05-522db86a58d4950019121180a35c9b56d0d8af76
export YTSAURUS_IMAGE_25_1_99 ?= ${YTSAURUS_IMAGE}-nightly:dev-25.1-2025-09-30-da1f6800979c77f5a19f44dc18d970bb5a70ac03
export YTSAURUS_IMAGE_25_2_99 ?= ${YTSAURUS_IMAGE}-nightly:dev-25.2-2025-12-09-d1b3fc19add6314e39833c19ebb8487f2f734ff9
export YTSAURUS_IMAGE_25_3_99 ?= ${YTSAURUS_IMAGE}-nightly:dev-25.3-2026-01-29-e13f47bfc34bcd70e6b268e0e3778366606e059e
export YTSAURUS_IMAGE_25_4_99 ?= ${YTSAURUS_IMAGE}-nightly:dev-25.4-2026-01-30-337b71eda802fd46fb01780fbfe2d7a1ba757b25
export YTSAURUS_IMAGE_26_1_99 ?= ${YTSAURUS_IMAGE}-nightly:dev-2026-01-30-20d7d021364a7e544a8ea59c22f058e0f13f0d94

# https://github.com/ytsaurus/ytsaurus/pkgs/container/query-tracker
# https://github.com/ytsaurus/ytsaurus/pkgs/container/query-tracker-nightly
# skopeo inspect --format '{{range .RepoTags}}{{.}}\n{{end -}}' docker://ghcr.io/ytsaurus/query-tracker-nightly:dev-2025-07-23-be855609d82ae5c6952a6b5c8c4e1706d1b18232

export QUERY_TRACKER_IMAGE ?= ${YTSAURUS_REGISTRY}/query-tracker

export QUERY_TRACKER_VERSION_PAST ?= 0.0.10
export QUERY_TRACKER_VERSION_PREVIOUS ?= 0.0.10
export QUERY_TRACKER_VERSION_CURRENT ?= 0.0.11
export QUERY_TRACKER_VERSION_COMING ?= 0.0.11
export QUERY_TRACKER_VERSION_FUTURE ?= 0.1.2

export QUERY_TRACKER_IMAGE_0_1_2 ?= ${QUERY_TRACKER_IMAGE}:stable-0.1.2

export QUERY_TRACKER_IMAGE_NIGHTLY ?= ${QUERY_TRACKER_IMAGE}-nightly:dev-2026-01-30-14313a1279bd3f096e4d621722d06f4dd511ce98

# https://github.com/ytsaurus/ytsaurus/pkgs/container/strawberry
# https://github.com/ytsaurus/ytsaurus/pkgs/container/strawberry-nightly
# skopeo inspect --format '{{range .RepoTags}}{{.}}\n{{end -}}' docker://ghcr.io/ytsaurus/strawberry-nightly:dev-2025-07-23-8c5b9c9b908af8b2f883de051b62072e795d40d3

export STRAWBERRY_IMAGE ?= ${YTSAURUS_REGISTRY}/strawberry

export STRAWBERRY_VERSION_PAST ?= 0.0.13
export STRAWBERRY_VERSION_PREVIOUS ?= 0.0.14
export STRAWBERRY_VERSION_CURRENT ?= 0.0.15
export STRAWBERRY_VERSION_COMING ?= 0.0.15
export STRAWBERRY_VERSION_FUTURE ?= 0.0.15

export STRAWBERRY_IMAGE_NIGHTLY ?= ${STRAWBERRY_IMAGE}-nightly:dev-2026-01-30-8551ddfc8cba5a5c2cc72d1cd2de3f8cdc206280

# https://github.com/ytsaurus/ytsaurus/pkgs/container/chyt
# https://github.com/ytsaurus/ytsaurus/pkgs/container/chyt-nightly
# skopeo inspect --format '{{range .RepoTags}}{{.}}\n{{end -}}' docker://ghcr.io/ytsaurus/chyt-nightly:dev-2025-07-23-85732937466aec6368c1582345afdedff70c5007

export CHYT_IMAGE ?= ${YTSAURUS_REGISTRY}/chyt

export CHYT_VERSION_PAST ?= 2.16.0
export CHYT_VERSION_PREVIOUS ?= 2.16.0
export CHYT_VERSION_CURRENT ?= 2.17.4
export CHYT_VERSION_COMING ?= 2.17.4
export CHYT_VERSION_FUTURE ?= 2.17.4

export CHYT_IMAGE_NIGHTLY ?= ${CHYT_IMAGE}-nightly:dev-2026-01-30-14313a1279bd3f096e4d621722d06f4dd511ce98

define LOAD_TEST_IMAGES
$(YTSAURUS_IMAGE_$(subst .,_,$(YTSAURUS_VERSION_PREVIOUS)))
$(YTSAURUS_IMAGE_$(subst .,_,$(YTSAURUS_VERSION_CURRENT)))
$(YTSAURUS_IMAGE_$(subst .,_,$(YTSAURUS_VERSION_COMING)))
${QUERY_TRACKER_IMAGE}:${QUERY_TRACKER_VERSION_CURRENT}
${CHYT_IMAGE}:${CHYT_VERSION_CURRENT}
endef

YTSAURUS_SAMPLE_IMAGE_CURRENT ?= ghcr.io/ytsaurus/ytsaurus:stable-25.2.2-relwithdebinfo

define LOAD_SAMPLE_IMAGES
${YTSAURUS_SAMPLE_IMAGE_CURRENT}
endef
