# Docker: Building images with security in mind

When it comes to running our microservices in production, we need to build docker
images. Security is unfortunately an afterthought, so let's try to figure out what
we can do to increase security so it's better than most of the stuff out there.

## Configuring Makefile targets

We will start by adding Makefile rules to enable us to build and push docker images.

~~~Makefile
# docker image build

IMAGE_PREFIX := titpetric/service-

docker: $(shell ls -d cmd/* | sed -e 's/cmd\//docker./')
	@echo OK.

docker.%: export SERVICE = $(shell basename $*)
docker.%:
	@figlet $(SERVICE)
	docker build --rm --no-cache -t $(IMAGE_PREFIX)$(SERVICE) --build-arg service_name=$(SERVICE) -f docker/serve/Dockerfile .

# docker image push

push: $(shell ls -d cmd/* | sed -e 's/cmd\//push./')
	@echo OK.

push.%: export SERVICE = $(shell basename $*)
push.%:
	@figlet $(SERVICE)
	docker push $(IMAGE_PREFIX)$(SERVICE)
~~~

There's nothing magical about this target, it's the same principle which we are already using for
our code generation, and service building. To use it, you can just run `make docker` to build the images,
and `make push` to push the images to docker hub or your registry, based on the IMAGE_PREFIX value.

## The basic Dockerfile image

Let's create our basic service Dockerfile under `docker/serve/Dockerfile`. We will start with some
reasonable defaults and then try to improve the security of the built images even further.

~~~Dockerfile
FROM alpine:latest

ARG service_name
ENV service_name=$service_name

WORKDIR /app

ENV TZ Europe/Ljubljana
RUN apk --no-cache add ca-certificates tzdata && ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone

COPY /build/${service_name}-linux-amd64 /app/service

RUN adduser -S www-data -u 1000
USER www-data

EXPOSE 3000

ENTRYPOINT ["/app/service"]
~~~

We are actually building a fully featured distro image on alpine here. We add the few required packages that
actually enable our service to function on some common baseline:

- `ca-certificates` adds the SSL certificates required for HTTPS, so you can consume public APIs,
- `tzdata` configures the timezone (usually optional if your apps will be built on UTC)

Security wise, we are doing two things:

- we add an unprivileged `www-data` user, so our service doesn't run as `root`,
- we run our service on port 3000 (a privileged user would be required to run on port 80).

The attack surface here requires that a person should first exploit our service, and then exploit the
running kernel to escalate privileges from www-data to root, and then could use `apk` to install packages
that could either do whatever inside the container, or attempt to break out of docker to own your host.

Can we improve this further? I believe we can. Two things come to mind:

- we can remove tools and binaries that enable `root` to have an usable container (remove `apk` to start),
- instead of relying on alpine, we could build our images on `scratch`, which is absolutely empty.

## Security implications of our docker image

In order to figure out all the files that are bundled in the container, we can use docker to
list the files built in the image. Since the container has `find`, this can be done with:

~~~bash
docker run --rm --entrypoint=/usr/bin/find titpetric/service-stats / -type f > files.txt
~~~

Inspecting the file, we can list the root folders to figure out which are safe and unsafe:

~~~plaintext
# cat files.txt | perl -p -e 's/^(\/[^\/]+).+/\1/g' | sort | uniq -c | sort -nr
   9904 /sys
   1362 /usr
   1130 /proc
     51 /etc
      8 /lib
      3 /sbin
      1 /.dockeren
      1 /bin
      1 /app
~~~

We can immediately ignore the `/sys` and `/proc` folders, since these are coming from our docker
container environment and don't include executable files. We could just inspect the executables
found in the PATH environment:

~~~plaintext
# docker run -it --rm --entrypoint=/bin/sh titpetric/service-stats -c set | grep PATH
PATH='/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin'
~~~

Since all the executable files are contained in either the `bin` or `sbin` folders, we can
lis them particularly just by looking for `bin/` with grep:

~~~plaintext
# cat files.txt | grep bin/
/usr/bin/c_rehash
/usr/bin/posixtz
/usr/bin/iconv
/usr/bin/getent
/usr/bin/getconf
/usr/bin/ssl_client
/usr/bin/scanelf
/usr/sbin/zdump
/usr/sbin/update-ca-certificates
/usr/sbin/zic
/bin/busybox
/sbin/apk
/sbin/ldconfig
/sbin/mkmntdirs
~~~

Now, the simpler attack surface for privilege escalation needs something that's called a `setuid`
bit set on the executable. This means that an unprivileged user like `www-data` can run something
as root. You can inspect the binaries by invoking find with `-perm -4000` as the parameter. On our
host, we'll end up a list like this:

~~~plaintext
# find / -perm -4000
/usr/lib/openssh/ssh-keysign
/usr/lib/eject/dmcrypt-get-device
/usr/lib/dbus-1.0/dbus-daemon-launch-helper
/usr/bin/chfn
/usr/bin/sudo
/usr/bin/gpasswd
/usr/bin/newgrp
/usr/bin/passwd
/usr/bin/chsh
/bin/su
/bin/mount
/bin/umount
~~~

Alpine doesn't come with any setuid executables (at least with out current package selection), which
means the only way to exploit the running container would be to attack the kernel. From here, the attack
surface can still theoretically be limited to the running container, meaning an attacker could run anything
in the container with elevated root privileges. The other way would be to break out of the cgroup of the
running process, and effectively attack the host. We can defend against the first scenario, just by
cleaning up the executables in the built image. Let's just do that:

~~~Dockerfile
# delete all the bundled binaries on standard PATH locations
RUN rm -rf /bin /sbin /usr/bin /usr/sbin /usr/local/bin /usr/local/sbin
~~~

By cleaning these up, we can be sure that the attacker can't just run `/bin/busybox` under a privileged
process and end up with a shell to your container. There are other ways to improve the security of your
container, and ultimately the security issue might be in your app and the libraries that are compiled in,
so we can't be sure that there is absolutely no way to exploit it, but at least we came pretty damn close.

## Possible improvements

There are two notable projects that deal with container security which you might want to look at.

- [Clair](https://github.com/quay/clair) - a static analyzer for common vulnerabilities,
- [docker-slim](https://github.com/docker-slim/docker-slim) - merge image layers and remove unused files

With Clair, the intent of the project is to continously scan your images for newly published vulnerabilities.
It will not solve them, but it will let you know if you need to upgrade or remove some of the packages
that are contained in your container.

With docker-slim, the project uses static analysis for doing what we did above by hand. We know that
our application doesn't rely on anything under `bin/` or `sbin/` folders, so we could safely delete them
without impacting our service. Docker slim goes further than that, and not only removes binaries, but
rebuilds the complete image without referencing the base image, and deleting everything that may be unused,
possibly even stripping debug symbols from your app, and ending up with a tiny image which goal is both
optimized for size and security. With it, your final build image would be similar to what you would
get if you started with `FROM scratch`.