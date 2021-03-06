package constants

type Language string

const (
	NODEJS Language = "NODEJS"
	GOLANG Language = "GOLANG"
)

const (
	// NodejsDockerfile  = "FROM node:alpine \n workdir /app \n copy package.json . \n run npm install \n copy . . \n cmd [\"node\", \"index.js\"]"
	Dockerfile        = "FROM node:alpine \n WORKDIR /app \n RUN yarn global add serve \n COPY . . \n RUN echo $(ls -lash /app/) \n RUN unzip build.zip \n CMD [\"serve\", \"-p\", \"4000\", \"-s\", \"./build\"]"
	NodejsPackageJSON = "{\r\n  \"name\": \"user-code-worker\",\r\n  \"version\": \"1.0.0\",\r\n  \"main\": \"index.js\",\r\n  \"license\": \"MIT\",\r\n  \"dependencies\": {\r\n    \"express\": \"^4.17.1\"\r\n  }\r\n}\r\n"
	// Namespace           = "serverless"
	Namespace           = "default"
	RegistryCredentials = "qweqwe"
)

type BuildStatus string

const (
	Building     BuildStatus = "Building"
	BuildSuccess BuildStatus = "Success"
	BuildFailed  BuildStatus = "Failed"
	NotBuilt     BuildStatus = "NotBuilt"
)

type DeploymentStatus string

const (
	DeploymentFailed DeploymentStatus = "DeploymentFailed"
	Deployed         DeploymentStatus = "Deployed"
	Deploying        DeploymentStatus = "Deploying"
	RedeployRequired DeploymentStatus = "RedeployRequired"
	NotDeployed      DeploymentStatus = "NotDeployed"
)

type LastAction string

const (
	UpdateAction LastAction = "Update"
	DeployAction LastAction = "Deploy"
	BuildAction  LastAction = "Build"
	CreateAction LastAction = "Create"
)
