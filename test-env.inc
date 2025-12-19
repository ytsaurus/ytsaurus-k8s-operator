# vim: set filetype=make :

# https://ytsaurus.tech/docs/en/admin-guide/releases

YTSAURUS_REGISTRY = ghcr.io/ytsaurus

# options for e2e tests

# https://github.com/ytsaurus/ytsaurus/pkgs/container/ytsaurus
# https://github.com/ytsaurus/ytsaurus/pkgs/container/ytsaurus-nightly
# skopeo inspect --format '{{range .RepoTags}}{{.}}\n{{end -}}' docker://ghcr.io/ytsaurus/ytsaurus-nightly

export YTSAURUS_IMAGE_24_1 = ${YTSAURUS_REGISTRY}/ytsaurus:stable-24.1.0
# export YTSAURUS_IMAGE_24_1 = ${YTSAURUS_REGISTRY}/ytsaurus-nightly:dev-24.1-2025-06-18-dd4bd8bdc4d28993e7185b820930761540b29ae9

export YTSAURUS_IMAGE_24_2 = ${YTSAURUS_REGISTRY}/ytsaurus:stable-24.2.1
# export YTSAURUS_IMAGE_24_2 = ${YTSAURUS_REGISTRY}/ytsaurus-nightly:dev-24.2-2025-09-05-522db86a58d4950019121180a35c9b56d0d8af76

export YTSAURUS_IMAGE_25_1 = ${YTSAURUS_REGISTRY}/ytsaurus:stable-25.1.0
# export YTSAURUS_IMAGE_25_1 = ${YTSAURUS_REGISTRY}/ytsaurus-nightly:dev-25.1-2025-09-30-da1f6800979c77f5a19f44dc18d970bb5a70ac03

export YTSAURUS_IMAGE_25_2 = ${YTSAURUS_REGISTRY}/ytsaurus:stable-25.2.1
# export YTSAURUS_IMAGE_25_2 = ${YTSAURUS_REGISTRY}/ytsaurus-nightly:dev-25.2-2025-12-09-d1b3fc19add6314e39833c19ebb8487f2f734ff9

# export YTSAURUS_IMAGE_25_3 = ${YTSAURUS_REGISTRY}/ytsaurus:stable-25.3.0
export YTSAURUS_IMAGE_25_3 = ${YTSAURUS_REGISTRY}/ytsaurus-nightly:dev-25.3-2025-12-16-c872e2fcffbfb0396f543c3a70ced006b80df1cd

# export YTSAURUS_IMAGE_25_4 = ${YTSAURUS_REGISTRY}/ytsaurus:stable-25.4.0
export YTSAURUS_IMAGE_25_4 = ${YTSAURUS_REGISTRY}/ytsaurus-nightly:dev-25.4-2025-12-19-e68c113063a2f442f0ea4f8b58acac31d07d27e1

export YTSAURUS_IMAGE_PREVIOUS = ${YTSAURUS_IMAGE_24_1}
export YTSAURUS_IMAGE_CURRENT = ${YTSAURUS_IMAGE_24_2}
export YTSAURUS_IMAGE_FUTURE = ${YTSAURUS_IMAGE_25_2}
export YTSAURUS_IMAGE_NIGHTLY = ${YTSAURUS_REGISTRY}/ytsaurus-nightly:dev-2025-12-19-54476101b0d8c7d9dfa4c0173732494b49641334

# https://github.com/ytsaurus/ytsaurus/pkgs/container/query-tracker
# https://github.com/ytsaurus/ytsaurus/pkgs/container/query-tracker-nightly
# skopeo inspect --format '{{range .RepoTags}}{{.}}\n{{end -}}' docker://ghcr.io/ytsaurus/query-tracker-nightly:dev-2025-07-23-be855609d82ae5c6952a6b5c8c4e1706d1b18232
export QUERY_TRACKER_IMAGE_PREVIOUS = ${YTSAURUS_REGISTRY}/query-tracker:0.0.10
export QUERY_TRACKER_IMAGE_CURRENT = ${YTSAURUS_REGISTRY}/query-tracker:0.0.11
export QUERY_TRACKER_IMAGE_FUTURE = ${YTSAURUS_REGISTRY}/query-tracker:0.1.1
export QUERY_TRACKER_IMAGE_NIGHTLY = ${YTSAURUS_REGISTRY}/query-tracker-nightly:dev-2025-12-19-54476101b0d8c7d9dfa4c0173732494b49641334

# https://github.com/ytsaurus/ytsaurus/pkgs/container/strawberry
# https://github.com/ytsaurus/ytsaurus/pkgs/container/strawberry-nightly
# skopeo inspect --format '{{range .RepoTags}}{{.}}\n{{end -}}' docker://ghcr.io/ytsaurus/strawberry-nightly:dev-2025-07-23-8c5b9c9b908af8b2f883de051b62072e795d40d3
export STRAWBERRY_IMAGE_PREVIOUS = ${YTSAURUS_REGISTRY}/strawberry:0.0.13
export STRAWBERRY_IMAGE_CURRENT = ${YTSAURUS_REGISTRY}/strawberry:0.0.14
export STRAWBERRY_IMAGE_FUTURE = ${YTSAURUS_REGISTRY}/strawberry:0.0.15
export STRAWBERRY_IMAGE_NIGHTLY = ${YTSAURUS_REGISTRY}/strawberry-nightly:dev-2025-12-19-aad5a97990f371aacb21eb2066cce24c46aa7c57

# https://github.com/ytsaurus/ytsaurus/pkgs/container/chyt
# https://github.com/ytsaurus/ytsaurus/pkgs/container/chyt-nightly
# skopeo inspect --format '{{range .RepoTags}}{{.}}\n{{end -}}' docker://ghcr.io/ytsaurus/chyt-nightly:dev-2025-07-23-85732937466aec6368c1582345afdedff70c5007
export CHYT_IMAGE_PREVIOUS = ${YTSAURUS_REGISTRY}/chyt:2.16.0
export CHYT_IMAGE_CURRENT = ${YTSAURUS_REGISTRY}/chyt:2.16.0
export CHYT_IMAGE_FUTURE = ${YTSAURUS_REGISTRY}/chyt:2.17.4
export CHYT_IMAGE_NIGHTLY = ${YTSAURUS_REGISTRY}/chyt-nightly:dev-2025-12-19-54476101b0d8c7d9dfa4c0173732494b49641334

define LOAD_TEST_IMAGES
${YTSAURUS_IMAGE_PREVIOUS}
${YTSAURUS_IMAGE_CURRENT}
${YTSAURUS_IMAGE_FUTURE}
${QUERY_TRACKER_IMAGE_CURRENT}
${CHYT_IMAGE_CURRENT}
endef

YTSAURUS_SAMPLE_IMAGE_CURRENT = ghcr.io/ytsaurus/ytsaurus:stable-25.2.1-relwithdebinfo

define LOAD_SAMPLE_IMAGES
${YTSAURUS_SAMPLE_IMAGE_CURRENT}
endef
