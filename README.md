# Static Site Hosting
Static site hosting in Kubernetes

## Installation and Usage

### To Run as Standalone

- Clone the repository
- Install skaffold if you haven't already
- Use `skaffold dev` to run in dev mode or use `skaffold run` to run without auto-reload

### To run Cloudbase fully 

Checkout the Cloudbase-main [repo](https://github.com/Cloudbase-Project/cloudbase-main)

## Implementation

**Tested only with React**

The architecture of the static site hosting service is very similar to the [serverless architecture](https://github.com/Cloudbase-Project/Serverless), reusing a lot of its components. We make use of the same Kaniko image building process, just changing the content of the image.

The user first creates a site object that contains metadata about the site itself. The user is then instructed to change the paths of the static files it uses. The user then zips the files and uploads them to cloudbase.

Once the zip file is uploaded, the service now adds the file to the worker queue. The worker queue is a queue containing sites that need to be built. The service then creates the kaniko worker pod, explained in detail in the serverless architecture, for building the image.

The init container of the pod downloads the zip file from the service based on the worker queue and places it in the shared volume for kaniko. The kaniko container then picks up the zip file, unzips it, builds the image and pushes it to the registry.

The deployment process is the same as the serverless component. Two kubernetes resources, Deployment and a ClusterIP service are used.

