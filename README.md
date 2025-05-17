<picture>
  <p style="text-align:center;" align="center">
<a href="https://meshery.io/">
<picture align="center">
<source media="(prefers-color-scheme: dark)" srcset=".github\readme\images\meshery-logo-dark-text-side.svg" width="70%" align="center" style="margin-bottom:20px;">
<source media="(prefers-color-scheme: light)" srcset=".github\readme\images\meshery-logo-light-text-side.svg" width="70%" align="center" style="margin-bottom:20px;">
<img alt="Shows an illustrated light mode meshery logo in light color mode and a dark mode meshery logo dark color mode." src=".github\readme\images\meshery-logo-light-text-side.svg" width="70%" align="center" style="margin-bottom:20px;"> </picture>
</a>

<br/><br/></p>
</picture>

[![Docker Pulls](https://img.shields.io/docker/pulls/layer5/meshsync.svg)](https://hub.docker.com/r/layer5/meshsync)
[![Go Report Card](https://goreportcard.com/badge/github.com/layer5io/meshsync)](https://goreportcard.com/report/github.com/layer5io/meshsync)
[![Build Status](https://img.shields.io/github/actions/workflow/status/meshery/meshsync/multi-platform.yml?branch=master)](https://github.com/meshery/meshsync/actions)
[![Website](https://img.shields.io/website/https/layer5.io/meshery.svg)](https://meshery.io/)
[![Twitter Follow](https://img.shields.io/twitter/follow/layer5.svg?label=Follow&style=social)](https://twitter.com/intent/follow?screen_name=mesheryio)
[![Slack](https://img.shields.io/badge/Slack-@layer5.svg?logo=slack)](http://slack.meshery.io)
[![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/3564/badge)](https://bestpractices.coreinfrastructure.org/projects/3564)
<a href="https://discuss.layer5.io/c/service-mesh-patterns/10" alt="Discuss Users">
  <img src="https://img.shields.io/discourse/users?label=discuss&logo=discourse&server=https%3A%2F%2Fdiscuss.layer5.io" /></a>
<br /><br />
<p style="text-align:center;" align="center"><a href="https://meshery.io"><img align="left" style="margin-bottom:20px;" src="https://raw.githubusercontent.com/layer5io/meshsync/master/.github/readme/images/meshsync.svg"  width="150px" /></a></p>

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

## Join the Community!

<a name="contributing"></a><a name="community"></a>
Our projects are community-built and welcome collaboration. 👍 Be sure to see the <a href="https://docs.meshery.io/project/contributing#not-sure-where-to-start">Contributor Welcome Guide</a> and <a href="https://meshery.io/community#handbook">Community Handbook</a> for a tour of resources available to you and the <a href="https://layer5.io/community/handbook/repository-overview">Repository Overview</a> for a cursory description of repository by technology and programming language. Jump into community <a href="https://slack.meshery.io">Slack</a> or <a href="https://meshery.io/community#discussion-forums">discussion forum</a> to participate.

<p style="clear:both;">
<a href ="https://meshery.io/community#meshmates"><img alt="MeshMates" src=".github\readme\images\layer5-community-sign.png" style="margin-right:10px; margin-bottom:7px;" width="28%" align="left" /></a>

<h3>Find your MeshMate</h3>

<p>MeshMates are experienced community members, who will help you learn your way around, discover live projects, and expand your community network. Connect with a Meshmate today!</p>

Learn more about the <a href="https://meshery.io/community#meshmates">MeshMates</a> program. <br />

</p>
<br /><br />
<div style="display: flex; justify-content: center; align-items:center;">
<div>
<a href="https://meshery.io/community"><img alt="Meshery Cloud Native Community" src="https://docs.meshery.io/assets/img/readme/community.png" width="140px" style="margin-right:36px; margin-bottom:7px;" width="140px" align="left"/></a>
</div>
<div style="width:60%; padding-left: 16px; padding-right: 16px">
<p>
✔️ <em><strong>Join</strong></em> any or all of the weekly meetings on <a href="https://meshery.io/calendar">community calendar</a>.<br />
✔️ <em><strong>Watch</strong></em> community <a href="https://www.youtube.com/@mesheryio?sub_confirmation=1">meeting recordings</a>.<br />
✔️ <em><strong>Fill-in</strong></em> a <a href="https://layer5.io/newcomers">member form</a> and gain access to community resources.
<br />
✔️ <em><strong>Discuss</strong></em> in the <a href="https://meshery.io/community#discussion-forums">Community Forum</a>.<br />
✔️ <em><strong>Explore more</strong></em> in the <a href="https://meshery.io/community#handbook">Community Handbook</a>.<br />
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
&nbsp;&nbsp;&nbsp;&nbsp; <i>Not sure where to start?</i> Grab an open issue with the <a href="https://github.com/issues?q=is%3Aopen+is%3Aissue+archived%3Afalse+org%3Alayer5io+org%3Ameshery+org%3Alayer5labs+org%3Aservice-mesh-performance+org%3Aservice-mesh-patterns+org%3Ameshery-extensions+label%3A%22help+wanted%22+">help-wanted label</a>.
</p>
<br /><br />

<div>&nbsp;</div>

## Contributing

Please do! We're a warm and welcoming community of open source contributors. Please join. All types of contributions are welcome. Be sure to read the [Contributor Guides](https://docs.meshery.io/project/contributing) for a tour of resources available to you and how to get started.
