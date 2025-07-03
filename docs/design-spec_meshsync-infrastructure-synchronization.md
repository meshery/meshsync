# MeshSync

## *Synchronizing Managed Infrastructure*

[**Design Prologue	4**](#design-prologue)

[**Design Goals	4**](#design-goals)

[Design Objectives	5](#design-objectives)

[**Concepts	5**](#concepts)

[MeshSync Functional Divisions / Components	5](#meshsync-functional-divisions-/-components)

[**Discovery	6**](#discovery)

[Composite Prints	6](#composite-prints)

[Tiered Discovery	7](#tiered-discovery)

[**User Stories	7**](#user-stories)

[Scenario: Greenfielding a cloud native infrastructure	7](#scenario:-greenfielding-a-cloud-native-infrastructure)

[User Story 1.1	7](#user-story-1.1)

[Scenario: Brownfielding a cloud native infrastructure	7](#scenario:-brownfielding-a-cloud-native-infrastructure)

[User Story 2.1	7](#user-story-2.1)

[**Messaging with NATS	8**](#messaging-with-nats)
# MeshSync

## *Synchronizing Managed Infrastructure*

[**Design Prologue	4**](#design-prologue)

[**Design Goals	4**](#design-goals)

[Design Objectives	5](#design-objectives)

[**Concepts	5**](#concepts)

[MeshSync Functional Divisions / Components	5](#meshsync-functional-divisions-/-components)

[**Discovery	6**](#discovery)

[Composite Prints	6](#composite-prints)

[Tiered Discovery	7](#tiered-discovery)

[**User Stories	7**](#user-stories)

[Scenario: Greenfielding a cloud native infrastructure	7](#scenario:-greenfielding-a-cloud-native-infrastructure)

[User Story 1.1	7](#user-story-1.1)

[Scenario: Brownfielding a cloud native infrastructure	7](#scenario:-brownfielding-a-cloud-native-infrastructure)

[User Story 2.1	7](#user-story-2.1)

[**Messaging with NATS	8**](#messaging-with-nats)

[Pipeline Notes	8](#pipeline-notes)

[Phase \-1	8](#phase--1)

[Architecture Diagram	8](#architecture-diagram)

[**Sequence Diagram	10**](#sequence-diagram)

[**Implementation Design Considerations	10**](#implementation-design-considerations)

[UI Framework	10](#ui-framework)

[Cytoscape.js	10](#cytoscape.js)

[**Object Model	10**](#object-model)

[**Questions	11**](#questions)

[MeshSync Discovery Funnel	12](#meshsync-discovery-funnel)

[Resources	12](#resources)

[Stages	12](#stages)

[**Design Architecture	13**](#design-architecture)

[Meshery Operator	13](#meshery-operator)

[Meshery Server	14](#meshery-server)

[Meshery Adapters / Controllers	14](#meshery-adapters-/-controllers)

[Graphql Subscriptions/mutations/queries	14](#graphql-subscriptions/mutations/queries)

[Sneak Peek into UI behavior:	15](#sneak-peek-into-ui-behavior:)

[When are Subscriptions flushed/re-instantiated?	16](#when-are-subscriptions-flushed/re-instantiated?)

[Object Models / Fingerprints	17](#object-models-/-fingerprints)

[Object: ClusterRoles / ClusterRoleBindings	17](#object:-clusterroles-/-clusterrolebindings)

[Object: Deployment	18](#object:-deployment)

[Object: Statefulsets	19](#object:-statefulsets)

[Object: ConfigMaps	19](#object:-configmaps)

| Resources: [https://github.com/layer5io/meshery-operator](https://github.com/layer5io/meshery-operator) |
| :---- |

## Design Prologue {#design-prologue}

MeshSync  
Cloud native infrastructure is dynamic. Changes in Kubernetes  and its workloads will occur out-of-band of Meshery. Operators won’t always use Meshery to interact with their infrastructure. Meshery will need to be continually cognizant of those changes.

1. **Meshery is not the source of authority for the state of an application**  
   The underlying infrastructure provider, like Kubernetes or a public cloud, like AWS is the source of authority. Meshery needs to be constantly updated in this regard. Meshery operations should be resilient in the face of this change.  
     
2. **An infrastructure agnostic object model**  
   At the heart of MeshSync will be an object model that defines relationships.

*Example Object Model*

Other example object models: 

* Cisco Intelligent Automation for Cloud

## Design Goals {#design-goals}

The designs in this specification should result in enabling:

**Support for containerized and non-containerized deployments:**

1. Support Kubernetes as a managed platform and public Clouds as managed platforms..  
2. Support Dockereployments.

**Support for Greenfield and Brownfield cloud native infrastructure deployments:**

1. Ability to scan the Kubernetes clusters to detect and identify various types of infrastructure, services, and applications deployed on the clusters.  
2. Ability to detect and distinguish services deployed on and off of cloud native infrastructurees.  
3. Cluster snapshot stored in-memory and refreshed in real-time in an event-based manner.  
4. Maintain a local snapshot of the cluster which is refreshed periodically (either through repeat scans or by watching the events stream from Kubernetes).

**Enable a visual topology:**

1. Ability to consistently show the cluster in its current state in UI using Kanvas  
2. Ability to let the end user make changes to the cluster through the UI.  
3. Ability to show the direction of traffic and the associated metrics on the chart for services.

**Be scalable and performant:**

1. Speed \- The implementation should be event-driven.   
2. Scale \- The implementation should support various controls around depth of object discovery.

### Design Objectives {#design-objectives}

The designs in this specification should result in these specific functions:

1. Creation of Meshery Operator and its custom resource definitions (CRDs).  
2. Custom controller using the client.go \`cache\` package.  
   1. Use Informers to be event-driven. Two types of Informers:  
      1. Index informers \- provide a key to a recently updated object.  
      2. Cache informers \- attach to a memory pool, running Watches. Caches are indexed.  
* Reflectors are types of Watchers (reflectors watch Kubernetes objects).   
* Queue is just an index of recently updated objects. Queue (FIFO) of updated objects and capable of being rate limited.  
* Converter deals with taking objects off the queue. Processing the elements from there is up to the custom controller.  
  3. Initial priming   
     4. Ongoing updates  
3. Implement discovery tiers (for speed and scalability of MeshSync) that successively refine the fingerprinting of objects and their changes.

   

Controller Runtime

## Concepts {#concepts}

### MeshSync Functional Divisions / Components {#meshsync-functional-divisions-/-components}

**Meshery Server (stateful)**

* Is the job scheduler \- occasionally invoke MeshSync   
* Is the discovery invoker \- ad hoc invoke MeshSync  
  * Calls the adapter to invoke discovery.  
* Add kubernetes support generically.


**Meshery Adapter (stateless)**

* **Meshery Common Library:** mechanism (e.g. interrogate Kubernetes, use the specific mesh’s client to interface with that mesh’s API)  
* Is the authority for identification (fingerprint).  
* Is the event listener


**Clients**

* mesheryctl, Meshery UI, or any consumer of the Meshery REST API.


**Tasks:**

1. Look at other implementations of watching kube-api.

# Discovery {#discovery}

## Composite Prints {#composite-prints}

Fingerprinting a cloud native infrastructure is the act of uniquely identifying managed infrastructure, their versions and other specific characteristics.

Use the same mechanisms that each infrastructure tool  uses to identify itself (e.g., istioctl version).

Number of proxies, and configuration of the proxies.

Identify the fingerprint for **Linkerd** using it’s CLI package? for **Consul**? for Kuma?

How to support individual versions of each cloud native infrastructure?

We should be able to assume backward compatibility within a given major release (e.g., within 1.x). Importing of packages 

Using a Builder pattern.

* Images  
* CRDs  
* Deployment

## Tiered Discovery {#tiered-discovery}

Kubernetes clusters may grow very large with thousands of objects on them. The process of identifying which objects are of  interest and which are not of interest can be intense.  Discovery tiers (for speed and scalability of MeshSync) successively refine the fingerprinting of infrasturcture and their changes.

**Discovery Phase 1:** kubectl get crds | grep “istio” || kubectl get deployments \--all-namespaces | grep “linkerd” ...  
**Discovery Phase 2:** k describe pods deploy/“istio” | grep “image \#”: istio-1.5.00.  
**Discovery Phase 3:** for any istio pods { query Istio’s api for version \#  }

## User Stories {#user-stories}

### Scenario: Greenfielding a cloud native infrastructure {#scenario:-greenfielding-a-cloud-native-infrastructure}

Meshery installing a cloud native infrastructure. Listening for provisioning notifications from kube-api.

#### User Story 1.1 {#user-story-1.1}

### Scenario: Brownfielding a cloud native infrastructure {#scenario:-brownfielding-a-cloud-native-infrastructure}

Meshery discovering a cloud native infrastructure. Searching kube-api for a specific or all cloud native infrastructures that are installed.

#### User Story 2.1  {#user-story-2.1}

Pod name istio an the container name istio

As an Operator,   
I would like to bring Meshery in as tooling post-deployment of my cloud native infrastructure,   
so that I can leverage Meshery’s functionality even though I didn’t create my cloud native infrastructure using Meshery to start with.

**Implementation:**

1. istio go client

**Acceptance Criteria:**

1. 

   

## Messaging with NATS  {#messaging-with-nats}

1. NATS will be a part of the controller deployment (Inside the cluster), such that if connectivity breaks, the results are persisted in the topics.

## Pipeline Notes {#pipeline-notes}

### Phase \-1 {#phase--1}

1. **Cluster component discovery in a single cluster**  
   Stages of discover:  
     
2. **Persisting past events to track back**

## Architecture Diagram {#architecture-diagram}

Draft:-  
![][image2]

## Sequence Diagram {#sequence-diagram}

\<here\>

## Implementation Design Considerations {#implementation-design-considerations}

### UI Framework {#ui-framework}

#### Cytoscape.js {#cytoscape.js}

This project is one of the best to work with network graphs. Nodes and edges are defined using a very simple model. Nodes are elements in an array and edges are elements with properties “from” and “to” which contain the node names.

All the other info we want to be presented in the UI or kept hidden can be added as metadatas to the nodes and edges. There is also a project react-cytoscape which makes it better to work with react.

## Object Model {#object-model}

Approach to the way in which the components (i.e. cloud native infrastructurees) under management are modeled.

There are 2 ways we can go about representing the cluster in the system:

1. Create a custom model which will keep the underlying orchestrator and cloud native infrastructure constructs abstracted  
2. Since we are mainly going to be working with cloud native infrastructurees on kubernetes, we can just not worry about adding any abstractions and just rely on Kubernetes model for representing the components including CRDs.

## Questions {#questions}

* How do we keep the Operator up to date with new cloud native infrastructure-specific custom resources (objects)? How do make MeshSync not fragile?  
* **\[Vinayak/Adheip\]** How to prime efficiently in large scale environments? How do we inspire confidence in Meshery users that Meshery will not be a bad actor on their existing infrastructure. How do we not overload Kubernetes with all of Meshery’s “discovery” (cache priming)?  
* Kubernetes cache pkg \- [https://github.com/kubernetes-sigs/controller-runtime/tree/master/pkg/cache](https://github.com/kubernetes-sigs/controller-runtime/tree/master/pkg/cache)   
* Adheip prototype \-  [https://github.com/AdheipSingh/khoj](https://github.com/AdheipSingh/khoj)   
* Under what use cases should we use the queue before processing an update?   
* Under what use cases should we interface with the cache directly?  
* **\[Vinayak\]** Are informers capable of watching custom resources?  
* **\[Vinayak\]** Start with a simple Watcher using client go’s shared worker pool example?  
  * Watches deployments and ...  
* Add the Calico custom controller example?  
* Controller will be deployed as a deployment and the pointer to Meshery server will be an environment variable in pod template spec or controlled through ConfigMap.  
* Meshery’s current implementation of MeshSync is in [kubernetes-scan.go](https://github.com/layer5io/meshery/blob/4aff1f3dfb787219f8d29b9f8fa039543a4bd03d/helpers/kubernetes-scan.go). We will want to rewrite \`[kubernetes-scan.go](https://github.com/layer5io/meshery/blob/4aff1f3dfb787219f8d29b9f8fa039543a4bd03d/helpers/kubernetes-scan.go)\` into this custom controller.


  


  

## MeshSync Discovery Funnel {#meshsync-discovery-funnel}

### Resources {#resources}

Global resources

- Nodes  
- Namespaces  
- Clusterroles/ClusterroleBindings  
- PersistentVolumes/Claims


  Namespace specific resources

- Services  
- Configmaps  
- Secrets  
- Deployments  
- Statefulset  
- Daemonsets  
- Jobs and cronjobs  
- Roles/RoleBindings  
- Pods     
- Replicaset

cloud native infrastructure specific discovery  
Istio:

- Control plane  
- Workloads  
- Custom Resource Destin

### Stages {#stages}

Stage \- 1 (Global resources discovery):

- Discover the global resources  
- Stream to NATS with resource ids

Stage \- 2 (Namespace specific discovery):

- Discover the namespace specific resources  
- Stream to NATS with resource ids

  Stage \- 3 (Mesh discovery):

- Every step will correspond to specific cloud native infrastructure discovery  
- Each step will spin its own pipeline only if the cloud native infrastructure exists in the cluster  
- Each and every step of the stage should end in order to get the results back  
- Each pipeline spun will run the cloud native infrastructure specific discovery  
- Adapters will be the subscribers of NATS for the data generated from this stage


  Questions:

1. How will the adapters know when to connect to NATS?  
   1. Initially we are going with infinite tries.  
2. cloud native infrastructure discovery in detail.  
3. Design a payload object for depicting the above.  
4. Design NATS object payload structure  
5. Who will fetch cluster specific data from NATS?  
   1. Meshery Server  
6. Discovery mechanism for these resources?  
7. Can informers be used outside of the Kubernetes cluster? (phrased differently can Kubernetes events be subscribed to outside of the cluster?)  
8. Is there a “dynamic” informer?  
   

# Design Architecture {#design-architecture}

The overall architecture consists of three major components:

- Meshery operator  
- Meshery server  
- Meshery adapters / Controllers


## Meshery Operator {#meshery-operator}

Meshery operator consists of kubernetes CRDs(Custom resource definitions) alongside several kubernetes custom controllers. This acts as a backbone to support several functionalities and use cases for meshery. Some of these functionalities include:

- Cluster discovery  
- Kubernetes and Cloud discovery for different types of  infrastructure  
- Data streaming via NATS


The custom resource definition inside the meshery-operator defines the fingerprinting for the cluster and mesh resources. Whenever a resource of this type is created, the MeshSync Controller is informed and it’s discovery process runs using the new fingerprint provided.

The Meshsync Controller is a custom kubernetes controller that performs the cluster and cloud native infrastructure discovery on the existing cluster. The object models, resource fingerprinting is done on demand from the meshery-server/meshery-adapter. This also is the producer application for the NATS subjects, to which meshery server/adapters are subscribed to.

## Meshery Server {#meshery-server}

Meshery Server is the subscriber application to NATS subjects in this architecture. It subscribes to those subjects that stream the cluster specific events and models. The subscribed data is persisted by this server on local/remote cache depending upon the user session.

## Meshery Adapters / Controllers {#meshery-adapters-/-controllers}

Meshery Adapters, soon to be Kubernetes custom controllers, are the subscriber applications to NATS subjects in this architecture. Each of the adapters  would subscribe to those subjects that stream their cloud native infrastructure specific events and models. 

Key points to be noted:

- Generic cluster specific fingerprints reside inside the meshery operator.  
- cloud native infrastructure specific fingerprints reside inside their respective adapters.  
- The NATS address/location is provided by the meshery server for the adapters to subscribe.  
- Meshery Operator initializes the NATS server (and set of subjects).

## Graphql Subscriptions/mutations/queries {#graphql-subscriptions/mutations/queries}

Key points to be noted for operator subscription:  
1\. Operator is deployed across all detected clusters. And operator status is continuously sent back to the client which can be \- ENABLED, DISABLED, PROCESSING.   
2\. \[All information is cluster specific\]Events that trigger Operator’s status to change(broadcaster is used here). The operatorSyncChannel is used to broadcast an event that signals to change the operator status in the existing subscription. This signal can set the state to processing or even send an error. If the signal is to not set the state in processing then we query kubeapi to get actual status of operator and if found then ENABLED is set. If the signal is to set the state in processing then operator’s state is set to PROCESSING no matter whether it is there or not i.e. we don't ask kubeapi. 

1. When  **meshery-operator, meshery-broker, meshery-meshsync** is detected in objects coming from Meshsync.  
2. When changeOperatorStatus is called, it signals to set the state in PROCESSING. When it's done, it resets state from PROCESSING and we query kubeapi again to get operator’s status.   
   3.\[All information is cluster specific\] Events that trigger data to change in control-plane and data-plane subscriptions.  
1. When Mesh Sync objects are detected in a given cluster.

###### *Sneak Peek into UI behavior:* {#sneak-peek-into-ui-behavior:}

The two subscriptions namely `operatorStatusSubscription and meshsyncStatusSubscription` are fired globally only once when the Meshery UI is mounted. 

 const operatorSubscription \= new GQLSubscription({ type : OPERATOR\_EVENT\_SUBSCRIPTION, contextIds : contexts, callbackFunction : operatorCallback })  
   const meshSyncSubscription \= new GQLSubscription({ type : MESHSYNC\_EVENT\_SUBSCRIPTION, contextIds : contexts, callbackFunction : meshSyncCallback })  
Link: [https://github.com/meshery/meshery/blob/master/ui/pages/\_app.js\#L120-L121](https://github.com/meshery/meshery/blob/master/ui/pages/_app.js#L120-L121)

Upon receipt, new subscription data is stored under global Redux variables located under: \`/ui/lib/store.js\`. These global state variables are intended for  use by any React component.  
   
Operator State:   
1\. ENABLED: The operator is fully opaque, and meshsync green signals that the context is in healthy state. Tooltip provides extended information.  
2\. DISABLED, PROCESSING: the navigator icon is 20% opaque and the tooltip with extended data to show the up-to-date state of operator.

The meshsyncstatus subscription behaves as similar as operator subscription.

###### *When are Subscriptions flushed/re-instantiated?* {#when-are-subscriptions-flushed/re-instantiated?}

The subscriptions are not flushed once instantiated, because it is not dependent on selected k8scontexts unlike other subscriptions. The operator states are subscribed for all the contexts in the kubeconfig

What infot the subscription carry?  
The following is the data operator comes with:

* data: {operator: {contextID: "8b15965edf3252e74dd037c8a9ee3559",…}}  
  * operator: {contextID: "8b15965edf3252e74dd037c8a9ee3559",…}  
    * contextID: "8b15965edf3252e74dd037c8a9ee3559"  
      * operatorStatus: {status: "ENABLED", version: "stable-latest",…}  
        * controllers: \[{name: "broker", version: "2.8.2-alpine3.15", status: ""},…\]  
          * 0: {name: "broker", version: "2.8.2-alpine3.15", status: ""}  
            * name: "broker"  
              * status: ""  
              * version: "2.8.2-alpine3.15"  
            * 1: {name: "meshsync", version: "stable-latest", status: "ENABLED 10.101.129.52:4222"}  
              * name: "meshsync"  
              * status: "ENABLED 10.101.129.52:4222"  
              * version: "stable-latest"  
          * error: null  
          * status: "ENABLED"  
          * version: "stable-latest"  
  * type: "data"  
  * 

It comes with contextId and operator data:  
Operator data contains: meshsync and nats status along with the operator status itself that shows the status of operator pod in meshery-namespace

## 

## Object Models / Fingerprints {#object-models-/-fingerprints}

Nodes

1. Meta  
2. ObjectMeta  
3. Spec  
   1. PodCIDR  
   2. PodCIDRs  
   3. ProviderId  
   4. Unschedulable  
4. Status  
   1. Capacity  
   2. Allocatable  
   3. Phase?  
   4. NodeInfo  
   5. Images  
   6. VolumeInUse  
   7. VolumesAttached  
   8. Config?

Namespaces

1. Meta  
2. ObjectMeta  
3. Spec  
   1. FInalizers  
4. Status  
   1. Phase?  
      

### Object: ClusterRoles / ClusterRoleBindings {#object:-clusterroles-/-clusterrolebindings}

PersistentVolumes

1. Meta  
2. ObjectMeta  
3. Spec?  
4. Status  
   1. Phase?  
   2. Message  
   3. Reason

PersistentVolumeClaim

1. Meta  
2. ObjectMeta  
3. Spec  
   1. AccessModes  
   2. Selector  
   3. Resources  
   4. VolumeName  
   5. StorageClassName  
   6. VolumeMode  
   7. DataSource  
4. Status  
   1. Phase?  
   2. AccessModes  
   3. Capacity

### Object: Deployment {#object:-deployment}

1. Meta  
   1. Kind  
   2. APIVersion  
2. ObjectMeta  
   1. Name  
   2. Namespace  
   3. UID  
   4. CreationTimestamp  
   5. DeletionTimeStamp  
   6. Labels  
   7. Annotations  
   8. ClusterName  
3. Spec  
   1. Replicas  
   2. Selector  
   3. Paused  
4. Status  
   1. Replicas  
   2. ReadyReplicas  
   3. AvailableReplicas  
   4. UnavailableReplicas  
   5. Conditions   
      1. Type  
      2. Status  
      3. LastUpdatedTime  
      4. LatTransitionTime  
      5. Reason  
      6. Message

      

### Object: Statefulsets {#object:-statefulsets}

1. Meta  
   1. Kind  
   2. APIVersion  
2. ObjectMeta  
   1. Name  
   2. Namespace  
   3. UID  
   4. CreationTimestamp  
   5. DeletionTimeStamp  
   6. Labels  
   7. Annotations  
   8. ClusterName  
3. Spec  
   1. Replicas  
   2. Selector  
   3. ServiceName  
4. Status  
   1. Replicas  
   2. ReadyReplicas  
   3. CurrentReplicas  
   4. UnavailableReplicas  
   5. Conditions  
      1. Type  
      2. Status  
      3. LastTransitionTime  
      4. Reason  
      5. Message

      

      \< Fill in core resources\>

### Object: ConfigMaps {#object:-configmaps}

1. Meta  
2. ObjectMeta  
3. Immutable  
4. DataMap  
5. BinaryMap?

   

   SERVICES:

1. Meta  
2. ObjectMeta  
3. ServiceSpec  
   1. Ports  
      1. Name  
      2. Protocol  
      3. AppProtocol  
      4. TargetPort  
      5. NodePort  
   2. Selector  
   3. ClusterIP  
   4. Type  
   5. ExternalIPs  
   6. LoadBalancerIP  
   7. LoadBalancerSourceRanges?  
   8. ExternalName  
   9. IPFamily ?  
   10. TopologyKeys ?  
4. ServiceStatus  
   1. LoadBalancer  
      SECRETS  
1. Meta  
2. ObjectMeta  
3. Immutable  
4. Data  
5. StringData  
6. Type

   

   DAEMONSETS

1. Meta  
2. ObjectMeta  
3. Spec  
   1. Selector  
   2. Template  
   3. UpdateStrategy  
   4. MinReadySeconds  
   5. RevisionHIstoryLimit  
4. Status  
   1. CurrentNumberScheduled  
   2. NumberMisscheduled  
   3. DesiredNumberScheduled  
   4. NumberReady  
   5. UpdatedNumberScheduled  
   6. NumberAvailable  
   7. NumberUnavailable

Jobs

1. Meta  
2. ObjectMeta  
3. Spec   
   1. Parallelism  
   2. Completions  
   3. ActiveDeadlineSeconds  
   4. BackoffLimit  
4. Status  
   1. StartTime  
   2. CompletionTime  
   3. Active  
   4. Succeeded  
   5. Failed

Roles

1. Meta  
2. ObjectMeta  
3. Rules  
   1. Verbs  
   2. APIGroups  
   3. Resources  
   4. ResourceNames  
   5. NonResourceURLs

RoleBindings

1. Meta  
2. ObjectMeta  
3. Subjects  
   1. Kind  
   2. APIGroup  
   3. Name  
   4. Namespace  
4. RoleRef  
   1. APIGroup  
   2. Kind  
   3. Name

Pods   

1. Meta  
2. ObjectMeta  
3. Spec?  
4. Status?

Replicaset

1. Meta  
2. ObjectMeta  
3. Spec  
   1. Replicas  
   2. Selectors2  
4. Status  
   1. Replicas  
   2. FullyLabeledReplicas  
   3. ReadyReplicas  
   4. AvailableReplicas

Istio-Discovery  
Networking   
Security

1\. Discovery all namespaces (and cache)  
Resources:  
Security

* Authorization policy   
* Peer Authentication    
* Request Authentication 

Networking

* Destination rules   
* Envoy Filters   
* Service Entries    
* Sidecars   
* Workload Entries   
* Workload Groups   
* Virtual services   
* Gateway 

[image1]: <data:image/png;base64,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>

[image2]: <data:image/png;base64,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>
[Pipeline Notes	8](#pipeline-notes)

[Phase \-1	8](#phase--1)

[Architecture Diagram	8](#architecture-diagram)

[**Sequence Diagram	10**](#sequence-diagram)

[**Implementation Design Considerations	10**](#implementation-design-considerations)

[UI Framework	10](#ui-framework)

[Cytoscape.js	10](#cytoscape.js)

[**Object Model	10**](#object-model)

[**Questions	11**](#questions)

[MeshSync Discovery Funnel	12](#meshsync-discovery-funnel)

[Resources	12](#resources)

[Stages	12](#stages)

[**Design Architecture	13**](#design-architecture)

[Meshery Operator	13](#meshery-operator)

[Meshery Server	14](#meshery-server)

[Meshery Adapters / Controllers	14](#meshery-adapters-/-controllers)

[Graphql Subscriptions/mutations/queries	14](#graphql-subscriptions/mutations/queries)

[Sneak Peek into UI behavior:	15](#sneak-peek-into-ui-behavior:)

[When are Subscriptions flushed/re-instantiated?	16](#when-are-subscriptions-flushed/re-instantiated?)

[Object Models / Fingerprints	17](#object-models-/-fingerprints)

[Object: ClusterRoles / ClusterRoleBindings	17](#object:-clusterroles-/-clusterrolebindings)

[Object: Deployment	18](#object:-deployment)

[Object: Statefulsets	19](#object:-statefulsets)

[Object: ConfigMaps	19](#object:-configmaps)

| Resources: [https://github.com/layer5io/meshery-operator](https://github.com/layer5io/meshery-operator) |
| :---- |

## Design Prologue {#design-prologue}

MeshSync  
Cloud native infrastructure is dynamic. Changes in Kubernetes  and its workloads will occur out-of-band of Meshery. Operators won’t always use Meshery to interact with their infrastructure. Meshery will need to be continually cognizant of those changes.

1. **Meshery is not the source of authority for the state of an application**  
   The underlying infrastructure provider, like Kubernetes or a public cloud, like AWS is the source of authority. Meshery needs to be constantly updated in this regard. Meshery operations should be resilient in the face of this change.  
     
2. **An infrastructure agnostic object model**  
   At the heart of MeshSync will be an object model that defines relationships.

*Example Object Model*

Other example object models: 

* Cisco Intelligent Automation for Cloud

## Design Goals {#design-goals}

The designs in this specification should result in enabling:

**Support for containerized and non-containerized deployments:**

1. Support Kubernetes as a managed platform and public Clouds as managed platforms..  
2. Support Dockereployments.

**Support for Greenfield and Brownfield cloud native infrastructure deployments:**

1. Ability to scan the Kubernetes clusters to detect and identify various types of infrastructure, services, and applications deployed on the clusters.  
2. Ability to detect and distinguish services deployed on and off of cloud native infrastructurees.  
3. Cluster snapshot stored in-memory and refreshed in real-time in an event-based manner.  
4. Maintain a local snapshot of the cluster which is refreshed periodically (either through repeat scans or by watching the events stream from Kubernetes).

**Enable a visual topology:**

1. Ability to consistently show the cluster in its current state in UI using Kanvas  
2. Ability to let the end user make changes to the cluster through the UI.  
3. Ability to show the direction of traffic and the associated metrics on the chart for services.

**Be scalable and performant:**

1. Speed \- The implementation should be event-driven.   
2. Scale \- The implementation should support various controls around depth of object discovery.

### Design Objectives {#design-objectives}

The designs in this specification should result in these specific functions:

1. Creation of Meshery Operator and its custom resource definitions (CRDs).  
2. Custom controller using the client.go \`cache\` package.  
   1. Use Informers to be event-driven. Two types of Informers:  
      1. Index informers \- provide a key to a recently updated object.  
      2. Cache informers \- attach to a memory pool, running Watches. Caches are indexed.  
* Reflectors are types of Watchers (reflectors watch Kubernetes objects).   
* Queue is just an index of recently updated objects. Queue (FIFO) of updated objects and capable of being rate limited.  
* Converter deals with taking objects off the queue. Processing the elements from there is up to the custom controller.  
  3. Initial priming   
     4. Ongoing updates  
3. Implement discovery tiers (for speed and scalability of MeshSync) that successively refine the fingerprinting of objects and their changes.

   

Controller Runtime

## Concepts {#concepts}

### MeshSync Functional Divisions / Components {#meshsync-functional-divisions-/-components}

**Meshery Server (stateful)**

* Is the job scheduler \- occasionally invoke MeshSync   
* Is the discovery invoker \- ad hoc invoke MeshSync  
  * Calls the adapter to invoke discovery.  
* Add kubernetes support generically.


**Meshery Adapter (stateless)**

* **Meshery Common Library:** mechanism (e.g. interrogate Kubernetes, use the specific mesh’s client to interface with that mesh’s API)  
* Is the authority for identification (fingerprint).  
* Is the event listener


**Clients**

* mesheryctl, Meshery UI, or any consumer of the Meshery REST API.


**Tasks:**

1. Look at other implementations of watching kube-api.

# Discovery {#discovery}

## Composite Prints {#composite-prints}

Fingerprinting a cloud native infrastructure is the act of uniquely identifying managed infrastructure, their versions and other specific characteristics.

Use the same mechanisms that each infrastructure tool  uses to identify itself (e.g., istioctl version).

Number of proxies, and configuration of the proxies.

Identify the fingerprint for **Linkerd** using it’s CLI package? for **Consul**? for Kuma?

How to support individual versions of each cloud native infrastructure?

We should be able to assume backward compatibility within a given major release (e.g., within 1.x). Importing of packages 

Using a Builder pattern.

* Images  
* CRDs  
* Deployment

## Tiered Discovery {#tiered-discovery}

Kubernetes clusters may grow very large with thousands of objects on them. The process of identifying which objects are of  interest and which are not of interest can be intense.  Discovery tiers (for speed and scalability of MeshSync) successively refine the fingerprinting of infrasturcture and their changes.

**Discovery Phase 1:** kubectl get crds | grep “istio” || kubectl get deployments \--all-namespaces | grep “linkerd” ...  
**Discovery Phase 2:** k describe pods deploy/“istio” | grep “image \#”: istio-1.5.00.  
**Discovery Phase 3:** for any istio pods { query Istio’s api for version \#  }

## User Stories {#user-stories}

### Scenario: Greenfielding a cloud native infrastructure {#scenario:-greenfielding-a-cloud-native-infrastructure}

Meshery installing a cloud native infrastructure. Listening for provisioning notifications from kube-api.

#### User Story 1.1 {#user-story-1.1}

### Scenario: Brownfielding a cloud native infrastructure {#scenario:-brownfielding-a-cloud-native-infrastructure}

Meshery discovering a cloud native infrastructure. Searching kube-api for a specific or all cloud native infrastructures that are installed.

#### User Story 2.1  {#user-story-2.1}

Pod name istio an the container name istio

As an Operator,   
I would like to bring Meshery in as tooling post-deployment of my cloud native infrastructure,   
so that I can leverage Meshery’s functionality even though I didn’t create my cloud native infrastructure using Meshery to start with.

**Implementation:**

1. istio go client

**Acceptance Criteria:**

1. 

   

## Messaging with NATS  {#messaging-with-nats}

1. NATS will be a part of the controller deployment (Inside the cluster), such that if connectivity breaks, the results are persisted in the topics.

## Pipeline Notes {#pipeline-notes}

### Phase \-1 {#phase--1}

1. **Cluster component discovery in a single cluster**  
   Stages of discover:  
     
2. **Persisting past events to track back**

## Architecture Diagram {#architecture-diagram}

Draft:-  
![][image2]

## Sequence Diagram {#sequence-diagram}

\<here\>

## Implementation Design Considerations {#implementation-design-considerations}

### UI Framework {#ui-framework}

#### Cytoscape.js {#cytoscape.js}

This project is one of the best to work with network graphs. Nodes and edges are defined using a very simple model. Nodes are elements in an array and edges are elements with properties “from” and “to” which contain the node names.

All the other info we want to be presented in the UI or kept hidden can be added as metadatas to the nodes and edges. There is also a project react-cytoscape which makes it better to work with react.

## Object Model {#object-model}

Approach to the way in which the components (i.e. cloud native infrastructurees) under management are modeled.

There are 2 ways we can go about representing the cluster in the system:

1. Create a custom model which will keep the underlying orchestrator and cloud native infrastructure constructs abstracted  
2. Since we are mainly going to be working with cloud native infrastructurees on kubernetes, we can just not worry about adding any abstractions and just rely on Kubernetes model for representing the components including CRDs.

## Questions {#questions}

* How do we keep the Operator up to date with new cloud native infrastructure-specific custom resources (objects)? How do make MeshSync not fragile?  
* **\[Vinayak/Adheip\]** How to prime efficiently in large scale environments? How do we inspire confidence in Meshery users that Meshery will not be a bad actor on their existing infrastructure. How do we not overload Kubernetes with all of Meshery’s “discovery” (cache priming)?  
* Kubernetes cache pkg \- [https://github.com/kubernetes-sigs/controller-runtime/tree/master/pkg/cache](https://github.com/kubernetes-sigs/controller-runtime/tree/master/pkg/cache)   
* Adheip prototype \-  [https://github.com/AdheipSingh/khoj](https://github.com/AdheipSingh/khoj)   
* Under what use cases should we use the queue before processing an update?   
* Under what use cases should we interface with the cache directly?  
* **\[Vinayak\]** Are informers capable of watching custom resources?  
* **\[Vinayak\]** Start with a simple Watcher using client go’s shared worker pool example?  
  * Watches deployments and ...  
* Add the Calico custom controller example?  
* Controller will be deployed as a deployment and the pointer to Meshery server will be an environment variable in pod template spec or controlled through ConfigMap.  
* Meshery’s current implementation of MeshSync is in [kubernetes-scan.go](https://github.com/layer5io/meshery/blob/4aff1f3dfb787219f8d29b9f8fa039543a4bd03d/helpers/kubernetes-scan.go). We will want to rewrite \`[kubernetes-scan.go](https://github.com/layer5io/meshery/blob/4aff1f3dfb787219f8d29b9f8fa039543a4bd03d/helpers/kubernetes-scan.go)\` into this custom controller.


  


  

## MeshSync Discovery Funnel {#meshsync-discovery-funnel}

### Resources {#resources}

Global resources

- Nodes  
- Namespaces  
- Clusterroles/ClusterroleBindings  
- PersistentVolumes/Claims


  Namespace specific resources

- Services  
- Configmaps  
- Secrets  
- Deployments  
- Statefulset  
- Daemonsets  
- Jobs and cronjobs  
- Roles/RoleBindings  
- Pods     
- Replicaset

cloud native infrastructure specific discovery  
Istio:

- Control plane  
- Workloads  
- Custom Resource Destin

### Stages {#stages}

Stage \- 1 (Global resources discovery):

- Discover the global resources  
- Stream to NATS with resource ids

Stage \- 2 (Namespace specific discovery):

- Discover the namespace specific resources  
- Stream to NATS with resource ids

  Stage \- 3 (Mesh discovery):

- Every step will correspond to specific cloud native infrastructure discovery  
- Each step will spin its own pipeline only if the cloud native infrastructure exists in the cluster  
- Each and every step of the stage should end in order to get the results back  
- Each pipeline spun will run the cloud native infrastructure specific discovery  
- Adapters will be the subscribers of NATS for the data generated from this stage


  Questions:

1. How will the adapters know when to connect to NATS?  
   1. Initially we are going with infinite tries.  
2. cloud native infrastructure discovery in detail.  
3. Design a payload object for depicting the above.  
4. Design NATS object payload structure  
5. Who will fetch cluster specific data from NATS?  
   1. Meshery Server  
6. Discovery mechanism for these resources?  
7. Can informers be used outside of the Kubernetes cluster? (phrased differently can Kubernetes events be subscribed to outside of the cluster?)  
8. Is there a “dynamic” informer?  
   

# Design Architecture {#design-architecture}

The overall architecture consists of three major components:

- Meshery operator  
- Meshery server  
- Meshery adapters / Controllers


## Meshery Operator {#meshery-operator}

Meshery operator consists of kubernetes CRDs(Custom resource definitions) alongside several kubernetes custom controllers. This acts as a backbone to support several functionalities and use cases for meshery. Some of these functionalities include:

- Cluster discovery  
- Kubernetes and Cloud discovery for different types of  infrastructure  
- Data streaming via NATS


The custom resource definition inside the meshery-operator defines the fingerprinting for the cluster and mesh resources. Whenever a resource of this type is created, the MeshSync Controller is informed and it’s discovery process runs using the new fingerprint provided.

The Meshsync Controller is a custom kubernetes controller that performs the cluster and cloud native infrastructure discovery on the existing cluster. The object models, resource fingerprinting is done on demand from the meshery-server/meshery-adapter. This also is the producer application for the NATS subjects, to which meshery server/adapters are subscribed to.

## Meshery Server {#meshery-server}

Meshery Server is the subscriber application to NATS subjects in this architecture. It subscribes to those subjects that stream the cluster specific events and models. The subscribed data is persisted by this server on local/remote cache depending upon the user session.

## Meshery Adapters / Controllers {#meshery-adapters-/-controllers}

Meshery Adapters, soon to be Kubernetes custom controllers, are the subscriber applications to NATS subjects in this architecture. Each of the adapters  would subscribe to those subjects that stream their cloud native infrastructure specific events and models. 

Key points to be noted:

- Generic cluster specific fingerprints reside inside the meshery operator.  
- cloud native infrastructure specific fingerprints reside inside their respective adapters.  
- The NATS address/location is provided by the meshery server for the adapters to subscribe.  
- Meshery Operator initializes the NATS server (and set of subjects).

## Graphql Subscriptions/mutations/queries {#graphql-subscriptions/mutations/queries}

Key points to be noted for operator subscription:  
1\. Operator is deployed across all detected clusters. And operator status is continuously sent back to the client which can be \- ENABLED, DISABLED, PROCESSING.   
2\. \[All information is cluster specific\]Events that trigger Operator’s status to change(broadcaster is used here). The operatorSyncChannel is used to broadcast an event that signals to change the operator status in the existing subscription. This signal can set the state to processing or even send an error. If the signal is to not set the state in processing then we query kubeapi to get actual status of operator and if found then ENABLED is set. If the signal is to set the state in processing then operator’s state is set to PROCESSING no matter whether it is there or not i.e. we don't ask kubeapi. 

1. When  **meshery-operator, meshery-broker, meshery-meshsync** is detected in objects coming from Meshsync.  
2. When changeOperatorStatus is called, it signals to set the state in PROCESSING. When it's done, it resets state from PROCESSING and we query kubeapi again to get operator’s status.   
   3.\[All information is cluster specific\] Events that trigger data to change in control-plane and data-plane subscriptions.  
1. When Mesh Sync objects are detected in a given cluster.

###### *Sneak Peek into UI behavior:* {#sneak-peek-into-ui-behavior:}

The two subscriptions namely `operatorStatusSubscription and meshsyncStatusSubscription` are fired globally only once when the Meshery UI is mounted. 

 const operatorSubscription \= new GQLSubscription({ type : OPERATOR\_EVENT\_SUBSCRIPTION, contextIds : contexts, callbackFunction : operatorCallback })  
   const meshSyncSubscription \= new GQLSubscription({ type : MESHSYNC\_EVENT\_SUBSCRIPTION, contextIds : contexts, callbackFunction : meshSyncCallback })  
Link: [https://github.com/meshery/meshery/blob/master/ui/pages/\_app.js\#L120-L121](https://github.com/meshery/meshery/blob/master/ui/pages/_app.js#L120-L121)

Upon receipt, new subscription data is stored under global Redux variables located under: \`/ui/lib/store.js\`. These global state variables are intended for  use by any React component.  
   
Operator State:   
1\. ENABLED: The operator is fully opaque, and meshsync green signals that the context is in healthy state. Tooltip provides extended information.  
2\. DISABLED, PROCESSING: the navigator icon is 20% opaque and the tooltip with extended data to show the up-to-date state of operator.

The meshsyncstatus subscription behaves as similar as operator subscription.

###### *When are Subscriptions flushed/re-instantiated?* {#when-are-subscriptions-flushed/re-instantiated?}

The subscriptions are not flushed once instantiated, because it is not dependent on selected k8scontexts unlike other subscriptions. The operator states are subscribed for all the contexts in the kubeconfig

What infot the subscription carry?  
The following is the data operator comes with:

* data: {operator: {contextID: "8b15965edf3252e74dd037c8a9ee3559",…}}  
  * operator: {contextID: "8b15965edf3252e74dd037c8a9ee3559",…}  
    * contextID: "8b15965edf3252e74dd037c8a9ee3559"  
      * operatorStatus: {status: "ENABLED", version: "stable-latest",…}  
        * controllers: \[{name: "broker", version: "2.8.2-alpine3.15", status: ""},…\]  
          * 0: {name: "broker", version: "2.8.2-alpine3.15", status: ""}  
            * name: "broker"  
              * status: ""  
              * version: "2.8.2-alpine3.15"  
            * 1: {name: "meshsync", version: "stable-latest", status: "ENABLED 10.101.129.52:4222"}  
              * name: "meshsync"  
              * status: "ENABLED 10.101.129.52:4222"  
              * version: "stable-latest"  
          * error: null  
          * status: "ENABLED"  
          * version: "stable-latest"  
  * type: "data"  
  * 

It comes with contextId and operator data:  
Operator data contains: meshsync and nats status along with the operator status itself that shows the status of operator pod in meshery-namespace

## 

## Object Models / Fingerprints {#object-models-/-fingerprints}

Nodes

1. Meta  
2. ObjectMeta  
3. Spec  
   1. PodCIDR  
   2. PodCIDRs  
   3. ProviderId  
   4. Unschedulable  
4. Status  
   1. Capacity  
   2. Allocatable  
   3. Phase?  
   4. NodeInfo  
   5. Images  
   6. VolumeInUse  
   7. VolumesAttached  
   8. Config?

Namespaces

1. Meta  
2. ObjectMeta  
3. Spec  
   1. FInalizers  
4. Status  
   1. Phase?  
      

### Object: ClusterRoles / ClusterRoleBindings {#object:-clusterroles-/-clusterrolebindings}

PersistentVolumes

1. Meta  
2. ObjectMeta  
3. Spec?  
4. Status  
   1. Phase?  
   2. Message  
   3. Reason

PersistentVolumeClaim

1. Meta  
2. ObjectMeta  
3. Spec  
   1. AccessModes  
   2. Selector  
   3. Resources  
   4. VolumeName  
   5. StorageClassName  
   6. VolumeMode  
   7. DataSource  
4. Status  
   1. Phase?  
   2. AccessModes  
   3. Capacity

### Object: Deployment {#object:-deployment}

1. Meta  
   1. Kind  
   2. APIVersion  
2. ObjectMeta  
   1. Name  
   2. Namespace  
   3. UID  
   4. CreationTimestamp  
   5. DeletionTimeStamp  
   6. Labels  
   7. Annotations  
   8. ClusterName  
3. Spec  
   1. Replicas  
   2. Selector  
   3. Paused  
4. Status  
   1. Replicas  
   2. ReadyReplicas  
   3. AvailableReplicas  
   4. UnavailableReplicas  
   5. Conditions   
      1. Type  
      2. Status  
      3. LastUpdatedTime  
      4. LatTransitionTime  
      5. Reason  
      6. Message

      

### Object: Statefulsets {#object:-statefulsets}

1. Meta  
   1. Kind  
   2. APIVersion  
2. ObjectMeta  
   1. Name  
   2. Namespace  
   3. UID  
   4. CreationTimestamp  
   5. DeletionTimeStamp  
   6. Labels  
   7. Annotations  
   8. ClusterName  
3. Spec  
   1. Replicas  
   2. Selector  
   3. ServiceName  
4. Status  
   1. Replicas  
   2. ReadyReplicas  
   3. CurrentReplicas  
   4. UnavailableReplicas  
   5. Conditions  
      1. Type  
      2. Status  
      3. LastTransitionTime  
      4. Reason  
      5. Message

      

      \< Fill in core resources\>

### Object: ConfigMaps {#object:-configmaps}

1. Meta  
2. ObjectMeta  
3. Immutable  
4. DataMap  
5. BinaryMap?

   

   SERVICES:

1. Meta  
2. ObjectMeta  
3. ServiceSpec  
   1. Ports  
      1. Name  
      2. Protocol  
      3. AppProtocol  
      4. TargetPort  
      5. NodePort  
   2. Selector  
   3. ClusterIP  
   4. Type  
   5. ExternalIPs  
   6. LoadBalancerIP  
   7. LoadBalancerSourceRanges?  
   8. ExternalName  
   9. IPFamily ?  
   10. TopologyKeys ?  
4. ServiceStatus  
   1. LoadBalancer  
      SECRETS  
1. Meta  
2. ObjectMeta  
3. Immutable  
4. Data  
5. StringData  
6. Type

   

   DAEMONSETS

1. Meta  
2. ObjectMeta  
3. Spec  
   1. Selector  
   2. Template  
   3. UpdateStrategy  
   4. MinReadySeconds  
   5. RevisionHIstoryLimit  
4. Status  
   1. CurrentNumberScheduled  
   2. NumberMisscheduled  
   3. DesiredNumberScheduled  
   4. NumberReady  
   5. UpdatedNumberScheduled  
   6. NumberAvailable  
   7. NumberUnavailable

Jobs

1. Meta  
2. ObjectMeta  
3. Spec   
   1. Parallelism  
   2. Completions  
   3. ActiveDeadlineSeconds  
   4. BackoffLimit  
4. Status  
   1. StartTime  
   2. CompletionTime  
   3. Active  
   4. Succeeded  
   5. Failed

Roles

1. Meta  
2. ObjectMeta  
3. Rules  
   1. Verbs  
   2. APIGroups  
   3. Resources  
   4. ResourceNames  
   5. NonResourceURLs

RoleBindings

1. Meta  
2. ObjectMeta  
3. Subjects  
   1. Kind  
   2. APIGroup  
   3. Name  
   4. Namespace  
4. RoleRef  
   1. APIGroup  
   2. Kind  
   3. Name

Pods   

1. Meta  
2. ObjectMeta  
3. Spec?  
4. Status?

Replicaset

1. Meta  
2. ObjectMeta  
3. Spec  
   1. Replicas  
   2. Selectors2  
4. Status  
   1. Replicas  
   2. FullyLabeledReplicas  
   3. ReadyReplicas  
   4. AvailableReplicas

Istio-Discovery  
Networking   
Security

1\. Discovery all namespaces (and cache)  
Resources:  
Security

* Authorization policy   
* Peer Authentication    
* Request Authentication 

Networking

* Destination rules   
* Envoy Filters   
* Service Entries    
* Sidecars   
* Workload Entries   
* Workload Groups   
* Virtual services   
* Gateway 

[image1]: <data:image/png;base64,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>

[image2]: <data:image/png;base64,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>
