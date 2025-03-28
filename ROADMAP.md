## Current ytsaurus-k8s-operator development roadmap

_As new versions are released, we will update tasks in this roadmap with corresponding versions and add new tasks._
***

- [x] Ability to configure logs in YTsaurus spec ([0.3.1](https://github.com/ytsaurus/ytsaurus-k8s-operator/releases/tag/release%2F0.3.1))
- [x] Logs rotation ([0.3.1](https://github.com/ytsaurus/ytsaurus-k8s-operator/releases/tag/release%2F0.3.1))
- [x] Release SPYT as a separate CRD ([0.3.1](https://github.com/ytsaurus/ytsaurus-k8s-operator/releases/tag/release%2F0.3.1))
- [x] Ability to create instance groups of proxies with different specification ([0.4.0](https://github.com/ytsaurus/ytsaurus-k8s-operator/releases/tag/release%2F0.4.0))
- [x] Ability to create instance groups of nodes with different specification ([0.4.0](https://github.com/ytsaurus/ytsaurus-k8s-operator/releases/tag/release%2F0.4.0))
- [x] Support of update scenario through snapshots and deleting tablet cells ([0.3.1](https://github.com/ytsaurus/ytsaurus-k8s-operator/releases/tag/release%2F0.3.1))
- [x] Namespace-scoped deploy mode ([0.18.0](https://github.com/ytsaurus/ytsaurus-k8s-operator/releases/tag/release%2F0.18.0))
- [ ] Support of some cluster reconfiguration scenarios (change volume mounts, instances count, etc)
- [ ] Support of CRI-O as CRI service sidecar
- [ ] Timbertruck Support for YTsaurus Log Delivery: Implement support for timbertruck to facilitate the delivery of YTsaurus logs into YTsaurus queues.
- [ ] Support for partial cluster update with granular component update selectors
- [ ] Support for rolling updates of cluster components
