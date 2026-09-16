# Infrastructure

initializing and validating terraform

```bash
terraform init
terraform validate
```

building and pushing the image:

```bash
gcloud artifacts repositories create albergo --repository-format=docker --location=europe-west3
gcloud builds submit --tag europe-west3-docker.pkg.dev/$PROJECT/albergo/app:$(git rev-parse --short HEAD)
```

one image serves both services: the worker overrides the entrypointwi:
`command = ["/worker"]`.
