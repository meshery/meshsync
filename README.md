<p style="text-align:center;" align="center"><a href="https://meshery.io"><picture>
 <source media="(prefers-color-scheme: dark)" srcset="https://raw.githubusercontent.com/meshery/meshery/master/.github/assets/images/readme/meshery-logo-light-text-side.svg">
 <source media="(prefers-color-scheme: light)" srcset="https://raw.githubusercontent.com/meshery/meshery/master/.github/assets/images/readme/meshery-logo-dark-text-side.svg">
<img src="https://raw.githubusercontent.com/meshery/meshery/master/.github/assets/images/readme/meshery-logo-dark-text-side.svg"
alt="Meshery Logo" width="70%" /></picture></a><br /><br /></p>
<p align="center">
<a href="https://hub.docker.com/r/meshery/meshery" alt="Docker pulls">
  <img src="https://img.shields.io/docker/pulls/meshery/meshery.svg" /></a>
<a href="https://github.com/issues?q=is%3Aopen%20is%3Aissue%20archived%3Afalse%20(org%3Ameshery%20OR%20org%3Aservice-mesh-performance%20OR%20org%3Aservice-mesh-patterns%20OR%20org%3Ameshery-extensions)%20label%3A%22help%20wanted%22%20" alt="GitHub issues by-label">
  <img src="https://img.shields.io/github/issues/meshery/meshery/help%20wanted.svg?color=informational" /></a>
<a href="https://github.com/meshery/meshery/blob/master/LICENSE" alt="LICENSE">
  <img src="https://img.shields.io/github/license/meshery/meshery?color=brightgreen" /></a>
<a href="https://artifacthub.io/packages/helm/meshery/meshery" alt="Artifact Hub Meshery">
  <img src="https://img.shields.io/endpoint?color=brightgreen&label=Helm%20Chart&style=plastic&url=https%3A%2F%2Fartifacthub.io%2Fbadge%2Frepository%2Fartifact-hub" /></a>  
<a href="https://goreportcard.com/report/github.com/meshery/meshery" alt="Go Report Card">
  <img src="https://goreportcard.com/badge/github.com/meshery/meshery" /></a>
<a href="https://github.com/meshery/meshery/actions" alt="Build Status">
  <img src="https://img.shields.io/github/actions/workflow/status/meshery/meshery/release-drafter.yml" /></a>
<a href="https://bestpractices.coreinfrastructure.org/projects/3564" alt="CLI Best Practices">
  <img src="https://bestpractices.coreinfrastructure.org/projects/3564/badge" /></a>
<a href="https://meshery.io/community#discussion-forums" alt="Discuss Users">
  <img src="https://img.shields.io/discourse/users?label=discuss&logo=discourse&server=https%3A%2F%2Fmeshery.io/community" /></a>
<a href="https://slack.meshery.io" alt="Join Slack">
  <img src="https://img.shields.io/badge/Slack-@meshery.svg?logo=slack" /></a>
<a href="https://twitter.com/intent/follow?screen_name=mesheryio" alt="Twitter Follow">
  <img src="https://img.shields.io/twitter/follow/mesheryio.svg?label=Follow+Meshery&style=social" /></a>
<a href="https://github.com/meshery/meshery/releases" alt="Meshery Downloads">
  <img src="https://img.shields.io/github/downloads/meshery/meshery/total" /></a>  
<a href="https://gurubase.io/g/meshery" alt="Meshery Guru">
  <img src="https://img.shields.io/badge/Gurubase-Ask%20Meshery%20Guru-006BFF" /></a>
<!-- <a href="https://app.fossa.com/projects/git%2Bgithub.com%2Fmeshery%2Fmeshery?ref=badge_shield" alt="License Scan Report">
  <img src="https://app.fossa.com/api/projects/git%2Bgithub.com%2Fmeshery%2Fmeshery.svg?type=shield"/></a>  
  -->
</p>

# MeshSync

MeshSync, an event-driven, continuous discovery and synchronization engine performs the task of ensuring that the state of configuration and status of operation of any supported Meshery platform (e.g. Kubernetes) and environment are known to Meshery Server. When deployed into Kubernetes enviroments, MeshSync runs as a Kubernetes custom controller under the control of Meshery Operator.

See [MeshSync in Meshery Docs](https://docs.meshery.io/concepts/architecture/meshsync) for additional information.

----

Could be run in two modes:
- nats (default)
- file

See details on input params in command help output:
```sh
meshsync --help
```


## NATS mode
NATS mode is the default mode.

In that mode, MeshSync expects a NATS connection and outputs Kubernetes resources updates into NATS queue, which is how MeshSync runs when deployed in Kubernetes cluster in conjuction with Meshery Broker.

## File mode
File mode is an option to run meshsync without dependency on nats and CRD.

In that mode meshsync outputs  k8s resources updates into file in kubernetes manifest yaml format.

The result of run is two files:
- meshery-cluster-snapshot-YYYYMMDD-00.yaml
- meshery-cluster-snapshot-YYYYMMDD-00-extended.yaml

meshery-cluster-snapshot-YYYYMMDD-00-extended.yaml contains all events meshsync produces as output;

meshery-cluster-snapshot-YYYYMMDD-00.yaml contains a deduplicated version where each resource is presented with one entity. 
Deduplication is done by `metadata.uid` field.


### Notes (on file mode)
Right now the format of the generated files is very close to kubernetes manifest yaml format, but not exactly the same. 

Generated files contain `metadata.labels` as array while in kubernetes manifest it should be an object.

It is due to the format of [KubernetesResourceObjectMeta](pkg/model/model.go#52).

`kubectl apply --dry-run` returns corresponding error:

```sh
kubectl apply --dry-run=client -f meshery-cluster-snapshot-YYYYMMDD-00.yaml

unable to decode "meshery-cluster-snapshot-YYYYMMDD-00.yaml": json: cannot unmarshal array into Go struct field ObjectMeta.metadata.labels of type map[string]string
```

<div>&nbsp;</div>

## Join the Meshery community

<a name="contributing"></a><a name="community"></a>
Our projects are community-built and welcome collaboration. 👍 Be sure to see the <a href="https://meshery.io/community">Contributor Journey Map</a> and <a href="https://meshery.io/community#handbook">Community Handbook</a> for a tour of resources available to you and the <a href="https://layer5.io/community/handbook/repository-overview">Repository Overview</a> for a cursory description of repository by technology and programming language. Jump into community <a href="https://slack.meshery.io">Slack</a> or <a href="https://meshery.io/community#discussion-forums">discussion forum</a> to participate.

<p style="clear:both;">
<h3>Find your MeshMate</h3>

<p>MeshMates are experienced Meshery community members, who will help you learn your way around, discover live projects, and expand your community network. Connect with a MeshMate today!</p>

Learn more about the <a href="https://meshery.io/community#meshmates">MeshMates</a> program. <br />

</p>
<br /><br />
<div style="display: flex; justify-content: center; align-items:center;">
<div>
<a href="https://meshery.io/community"><img alt="Meshery Community" src="https://docs.meshery.io/assets/img/readme/community.png" width="140px" style="margin-right:36px; margin-bottom:7px;" width="140px" align="left"/></a>
</div>
<div style="width:60%; padding-left: 16px; padding-right: 16px">
<p>
✔️ <em><strong>Join</strong></em> any or all of the weekly meetings on <a href="https://meshery.io/calendar">community calendar</a>.<br />
✔️ <em><strong>Watch</strong></em> community <a href="https://www.youtube.com/@mesheryio?sub_confirmation=1">meeting recordings</a>.<br />
✔️ <em><strong>Fill-in</strong></em> a <a href="https://meshery.io/newcomers">member form</a> and gain access to community resources.
<br />
✔️ <em><strong>Discuss</strong></em> in the <a href="https://meshery.io/community#discussion-forums">community forum</a>.<br />
✔️ <em><strong>Explore more</strong></em> in the <a href="https://meshery.io/community#handbook">community handbook</a>.<br />
</p>
</div><br /><br />
<div>
<a href="https://slack.meshery.io">
<picture>
  <source media="(prefers-color-scheme: dark)" srcset="https://raw.githubusercontent.com/meshery/meshery/master/.github/assets/images/readme/slack.svg"  width="110px" />
  <source media="(prefers-color-scheme: light)" srcset="https://raw.githubusercontent.com/meshery/meshery/master/.github/assets/images/readme/slack.svg" width="110px" />
  <img alt="Shows an illustrated light mode meshery logo in light color mode and a dark mode meshery logo dark color mode." src="https://raw.githubusercontent.com/meshery/meshery/master/.github/assets/images/readme/slack.svg" width="110px" align="left" />
</picture>
</a>
</div>
</div>
<br /><br />
<p align="left">
&nbsp;&nbsp;&nbsp;&nbsp; <i>Not sure where to start?</i> Grab an open issue with the <a href="https://github.com/issues?q=is%3Aopen%20is%3Aissue%20archived%3Afalse%20(org%3Ameshery%20OR%20org%3Aservice-mesh-performance%20OR%20org%3Aservice-mesh-patterns%20OR%20org%3Ameshery-extensions)%20label%3A%22help%20wanted%22%20">help-wanted label</a>.
</p>
<br /><br />

<div>&nbsp;</div>

## Contributing

Please do! We're a warm and welcoming community of open source contributors. Please join. All types of contributions are welcome. Be sure to read the [Contributor Guides](https://docs.meshery.io/project/contributing) for a tour of resources available to you and how to get started.

<!-- <a href="https://youtu.be/MXQV-i-Hkf8"><img alt="Deploying Linkerd with Meshery" src="https://docs.meshery.io/assets/img/readme/deploying-linkerd-with-meshery.png" width="100%" align="center" /></a> -->

<div>&nbsp;</div>

### Stargazers

<p align="center">
  <i>If you like Meshery, please <a href="../../stargazers">★</a> star this repository to show your support! 🤩</i>
 <br />
<a href="../../stargazers">
 <img align="center" src="https://api.star-history.com/svg?repos=meshery/meshery&type=Date" />
</a></p>

### License

This repository and site are available as open-source under the terms of the [Apache 2.0 License](https://opensource.org/licenses/Apache-2.0).