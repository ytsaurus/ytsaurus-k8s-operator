# vim: set filetype=make :

# https://github.com/ytsaurus/ytsaurus/pkgs/container/ytsaurus
# https://github.com/ytsaurus/ytsaurus/pkgs/container/ytsaurus-nightly

YTSAURUS_REGISTRY = ghcr.io/ytsaurus

# options for e2e tests

export YTSAURUS_IMAGE_23_2 = ${YTSAURUS_REGISTRY}/ytsaurus:stable-23.2.0
export YTSAURUS_IMAGE_24_1 = ${YTSAURUS_REGISTRY}/ytsaurus:stable-24.1.0
export YTSAURUS_IMAGE_24_2 = ${YTSAURUS_REGISTRY}/ytsaurus:stable-24.2.0

export YTSAURUS_IMAGE_PREVIOUS = ${YTSAURUS_IMAGE_23_2}
export YTSAURUS_IMAGE_CURRENT = ${YTSAURUS_IMAGE_24_1}
export YTSAURUS_IMAGE_FUTURE = ${YTSAURUS_IMAGE_24_2}

export YTSAURUS_IMAGE_NIGHTLY = ${YTSAURUS_REGISTRY}/ytsaurus-nightly:dev-24.2-2025-03-19-2973ab7cb36ed53ae3cbe9c37b8c7f55eb9c4e77

export QUERY_TRACKER_IMAGE_PREVIOUS = ${YTSAURUS_REGISTRY}/query-tracker:0.0.6
export QUERY_TRACKER_IMAGE_CURRENT = ${YTSAURUS_REGISTRY}/query-tracker:0.0.9
export QUERY_TRACKER_IMAGE_FUTURE = ${YTSAURUS_REGISTRY}/query-tracker:0.0.9

define LOAD_TEST_IMAGES
${YTSAURUS_IMAGE_23_2}
${YTSAURUS_IMAGE_24_1}
${YTSAURUS_IMAGE_24_2}
${YTSAURUS_IMAGE_NIGHTLY}
endef

YTSAURUS_SAMPLE_IMAGE_24_1 = ${YTSAURUS_REGISTRY}/ytsaurus:stable-24.1.0-relwithdebinfo

define LOAD_SAMPLE_IMAGES
${YTSAURUS_SAMPLE_IMAGE_24_1}
endef
