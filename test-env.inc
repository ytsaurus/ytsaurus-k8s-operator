# vim: set filetype=make :

# https://ytsaurus.tech/docs/en/admin-guide/releases

YTSAURUS_REGISTRY = ghcr.io/ytsaurus

# options for e2e tests

# https://github.com/ytsaurus/ytsaurus/pkgs/container/ytsaurus
# https://github.com/ytsaurus/ytsaurus/pkgs/container/ytsaurus-nightly
# skopeo inspect --format '{{range .RepoTags}}{{.}}\n{{end -}}' docker://ghcr.io/ytsaurus/ytsaurus-nightly

export YTSAURUS_IMAGE_23_2 = ${YTSAURUS_REGISTRY}/ytsaurus:stable-23.2.0
# export YTSAURUS_IMAGE_23_2 = ${YTSAURUS_REGISTRY}/ytsaurus-nightly:dev-23.2-2025-02-19-7dae008dd5171a9c89857f2034746ac3119842db

export YTSAURUS_IMAGE_24_1 = ${YTSAURUS_REGISTRY}/ytsaurus:stable-24.1.0
# export YTSAURUS_IMAGE_24_1 = ${YTSAURUS_REGISTRY}/ytsaurus-nightly:dev-24.1-2025-06-18-dd4bd8bdc4d28993e7185b820930761540b29ae9

export YTSAURUS_IMAGE_24_2 = ${YTSAURUS_REGISTRY}/ytsaurus:stable-24.2.1
# export YTSAURUS_IMAGE_24_2 = ${YTSAURUS_REGISTRY}/ytsaurus-nightly:dev-24.2-2025-07-28-567fa5b92fd448f708f84f7755dacf4560810ef6

# export YTSAURUS_IMAGE_25_1 = ${YTSAURUS_REGISTRY}/ytsaurus:stable-25.1.0 # missing
export YTSAURUS_IMAGE_25_1 = ${YTSAURUS_REGISTRY}/ytsaurus-nightly:dev-25.1-2025-07-28-b75342ada631c8d78b95c29c4de46225c877f1d2

# export YTSAURUS_IMAGE_25_2 = ${YTSAURUS_REGISTRY}/ytsaurus:stable-25.2.0 # missing
# export YTSAURUS_IMAGE_25_2 = ${YTSAURUS_REGISTRY}/ytsaurus-nightly:dev-25.2-2025-07-04-6f04dd8da0ad0b87ed992ac6f80b58b9ce04fbd3

export YTSAURUS_IMAGE_PREVIOUS = ${YTSAURUS_IMAGE_24_1}
export YTSAURUS_IMAGE_CURRENT = ${YTSAURUS_IMAGE_24_2}
export YTSAURUS_IMAGE_FUTURE = ${YTSAURUS_IMAGE_25_1}
# export YTSAURUS_IMAGE_NIGHTLY = ${YTSAURUS_REGISTRY}/ytsaurus-nightly:dev-2025-07-21-bc55d64bab08263dbc85557f4f537d2504bb6080

# https://github.com/ytsaurus/ytsaurus/pkgs/container/query-tracker
# https://github.com/ytsaurus/ytsaurus/pkgs/container/query-tracker-nightly
# skopeo inspect --format '{{range .RepoTags}}{{.}}\n{{end -}}' docker://ghcr.io/ytsaurus/query-tracker-nightly:dev-2025-07-23-be855609d82ae5c6952a6b5c8c4e1706d1b18232
export QUERY_TRACKER_IMAGE_PREVIOUS = ${YTSAURUS_REGISTRY}/query-tracker:0.0.9
export QUERY_TRACKER_IMAGE_CURRENT = ${YTSAURUS_REGISTRY}/query-tracker:0.0.10
# export QUERY_TRACKER_IMAGE_FUTURE = ${YTSAURUS_REGISTRY}/query-tracker:0.0.9
# export QUERY_TRACKER_IMAGE_NIGHTLY = ${YTSAURUS_REGISTRY}/query-tracker-nightly:dev-2025-07-23-54e157536780b978957c79559a88d605a42e0ca0

# https://github.com/ytsaurus/ytsaurus/pkgs/container/strawberry
# https://github.com/ytsaurus/ytsaurus/pkgs/container/strawberry-nightly
# skopeo inspect --format '{{range .RepoTags}}{{.}}\n{{end -}}' docker://ghcr.io/ytsaurus/strawberry-nightly:dev-2025-07-23-8c5b9c9b908af8b2f883de051b62072e795d40d3
export STRAWBERRY_IMAGE_PREVIOUS = ${YTSAURUS_REGISTRY}/strawberry:0.0.12
export STRAWBERRY_IMAGE_CURRENT = ${YTSAURUS_REGISTRY}/strawberry:0.0.13
# export STRAWBERRY_IMAGE_FUTURE = ${YTSAURUS_REGISTRY}/strawberry:0.0.13
# export STRAWBERRY_IMAGE_NIGHTLY = ${YTSAURUS_REGISTRY}/strawberry-nightly:dev-2025-07-23-0a27143f2b3962d3e25ef7b5822b9969d64aa2dc

# https://github.com/ytsaurus/ytsaurus/pkgs/container/chyt
# https://github.com/ytsaurus/ytsaurus/pkgs/container/chyt-nightly
# skopeo inspect --format '{{range .RepoTags}}{{.}}\n{{end -}}' docker://ghcr.io/ytsaurus/chyt-nightly:dev-2025-07-23-85732937466aec6368c1582345afdedff70c5007
export CHYT_IMAGE_PREVIOUS = ${YTSAURUS_REGISTRY}/chyt:2.16.0
export CHYT_IMAGE_CURRENT = ${YTSAURUS_REGISTRY}/chyt:2.17.2
# export CHYT_IMAGE_FUTURE = ${YTSAURUS_REGISTRY}/chyt:2.17.2
# export CHYT_IMAGE_NIGHTLY = ${YTSAURUS_REGISTRY}/chyt-nightly:dev-2025-07-23-85732937466aec6368c1582345afdedff70c5007

define LOAD_TEST_IMAGES
${YTSAURUS_IMAGE_23_2}
${YTSAURUS_IMAGE_PREVIOUS}
${YTSAURUS_IMAGE_CURRENT}
${YTSAURUS_IMAGE_FUTURE}
${QUERY_TRACKER_IMAGE_CURRENT}
${CHYT_IMAGE_CURRENT}
endef

YTSAURUS_SAMPLE_IMAGE_24_1 = ${YTSAURUS_REGISTRY}/ytsaurus:stable-24.1.0-relwithdebinfo

define LOAD_SAMPLE_IMAGES
${YTSAURUS_SAMPLE_IMAGE_24_1}
endef
