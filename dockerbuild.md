###Build dockerfile
```
$ docker build -t portalfeeders .
```

###Docker run with default environment
```
$ docker run -v ${PWD}/logs:/go/src/app/logs -d --name portal-feeders portalfeeders
```

###Docker run with environment
```
$ docker run --name portal-feeders -e "INCOGNITO_HOST=127.0.0.1" portalfeeders
```

###Docker container
```
$ docker start/restart/stop container_id
```