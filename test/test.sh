#!/bin/bash
# Basic integration tests with postgres. Requires docker to work.

SOURCE="${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ]; do # resolve $SOURCE until the file is no longer a symlink
  DIR="$( cd -P "$( dirname "$SOURCE" )" && pwd )"
  SOURCE="$(readlink "$SOURCE")"
  [[ $SOURCE != /* ]] && SOURCE="$DIR/$SOURCE" # if $SOURCE was a relative symlink, we need to resolve it relative to the path where the symlink file was located
done
DIR="$( cd -P "$( dirname "$SOURCE" )" && pwd )"

METRICS_DIR=$(pwd)

# Read the absolute path to the exporter
opengauss_exporter="$1"
config_file="$2"
export GS_PASSWORD=Ogexport@123
exporter_port=9187
opengauss_port=9188

echo "Exporter Binary: $opengauss_exporter" 1>&2

[ -z "$opengauss_exporter" ] && echo "Missing exporter binary" && exit 1

cd "$DIR" || exit 1

VERSIONS=( \
    1.0.1 \
)

wait_for_openGauss(){
    local container=$1
    local ip=$2
    local port=$3
    if [ -z "$ip" ]; then
        echo "No IP specified." 1>&2
        exit 1
    fi

    if [ -z "$port" ]; then
        echo "No port specified." 1>&2
        exit 1
    fi

    local wait_start
    wait_start=$(date +%s) || exit 1
    echo "Waiting for opengauss to start listening..."
    while true
    do
        aa=`docker logs "$container" 2>&1 |grep 'Success to start openGauss Database' |wc -l`;
        if [ $aa -gt 1 ]; then
          break
        fi
        if [ $(( $(date +%s) - wait_start )) -gt "$TIMEOUT" ]; then
            echo "Timed out waiting for postgres to start!" 1>&2
            exit 1
        fi
        sleep 1
    done
    echo "openGauss is online at $ip:$port"
}

wait_for_exporter() {
    local wait_start
    wait_start=$(date +%s) || exit 1
    echo "Waiting for exporter to start..."
    while ! nc -z localhost "$exporter_port" ; do
        if [ $(( $(date +%s) - wait_start )) -gt "$TIMEOUT" ]; then
            echo "Timed out waiting for exporter!" 1>&2
            exit 1
        fi
        sleep 1
    done
    echo "Exporter is online at localhost:$exporter_port"
}

smoke_test_opengauss() {
    local version=$1
    local CONTAINER_NAME=opengauss_exporter-test-smoke
    local TIMEOUT=90
    local IMAGE_NAME=enmotech/opengauss

    local CUR_IMAGE=$IMAGE_NAME:$version

    echo "###############################"
    echo " Standalone openGauss $version "
    echo "###############################"
    local docker_cmd="docker run -d -p $opengauss_port:5432 --privileged=true -e GS_PASSWORD=$GS_PASSWORD $CUR_IMAGE"
    echo "Docker Cmd: $docker_cmd"
    CONTAINER_NAME=$($docker_cmd)
    standalone_ip=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' $CONTAINER_NAME)
    # shellcheck disable=SC2064
    trap "docker logs $CONTAINER_NAME ; docker kill $CONTAINER_NAME ; docker rm -v $CONTAINER_NAME; exit 1" EXIT INT TERM
    wait_for_openGauss "$CONTAINER_NAME" "$standalone_ip" 5432

    # Extract a raw metric list.
    export DATA_SOURCE_NAME="postgresql://gaussdb:$GS_PASSWORD@localhost:$opengauss_port/postgres?sslmode=disable"
    exportCmd="$opengauss_exporter --log.level=debug --web.listen-address=:$exporter_port"
    if [ ! -z "$config_file" ]; then
      exportCmd=$exportCmd" --config=$config_file"
    fi
    echo "Exporter Cmd: $exportCmd"
    $exportCmd &
    exporter_pid=$!
    # shellcheck disable=SC2064
    trap "docker logs $CONTAINER_NAME ; docker kill $CONTAINER_NAME ; docker rm -v $CONTAINER_NAME; kill $exporter_pid; exit 1" EXIT INT TERM
    wait_for_exporter

    # Dump the metrics to a file.
    if ! wget -q -O - http://localhost:$exporter_port/metrics 1> "$METRICS_DIR/.metrics.single.$version.prom" ; then
        echo "Failed on openGauss $version (standalone $DOCKER_IMAGE)" 1>&2
        kill $exporter_pid
        exit 1
    fi

    # HACK test: check pg_up is a 1 - TODO: expand integration tests to include metric consumption
    if ! grep 'pg_up.* 1' $METRICS_DIR/.metrics.single.$version.prom ; then
        echo "pg_up metric was not 1 despite exporter and database being up"
        kill $exporter_pid
        exit 1
    fi
    if grep 'pg_exporter_last_scrape_error.* 1' $METRICS_DIR/.metrics.single.$version.prom ; then
        echo "pg_exporter_last_scrape_error metric was 1 despite exporter and database being up"
        kill $exporter_pid
        exit 1
    fi

    kill $exporter_pid
    docker kill "$CONTAINER_NAME"
    docker rm -v "$CONTAINER_NAME"
    trap - EXIT INT TERM

#    echo "#######################"
#    echo "Replicated openGauss $version"
#    echo "#######################"
#    old_pwd=$(pwd)
#    cd docker-postgres-replication || exit 1
#
#    if ! VERSION="$version" p2 -t Dockerfile.p2 -o Dockerfile ; then
#        echo "Templating failed" 1>&2
#        exit 1
#    fi
#    trap "docker-compose logs; docker-compose down ; docker-compose rm -v; exit 1" EXIT INT TERM
#    local compose_cmd="GS_PASSWORD=$GS_PASSWORD docker-compose up -d --force-recreate --build"
#    echo "Compose Cmd: $compose_cmd"
#    eval "$compose_cmd"
#
#    master_container=$(docker-compose ps -q pg-master)
#    slave_container=$(docker-compose ps -q pg-slave)
#    master_ip=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$master_container")
#    slave_ip=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$slave_container")
#    echo "Got master IP: $master_ip"
#    wait_for_openGauss "$master_container" "$master_ip" 5432
#    wait_for_openGauss "$slave_container" "$slave_ip" 5432
#
#    DATA_SOURCE_NAME="postgresql://postgres:$GS_PASSWORD@$master_ip:5432/?sslmode=disable" $test_binary || exit $?
#
#    DATA_SOURCE_NAME="postgresql://postgres:$GS_PASSWORD@$master_ip:5432/?sslmode=disable" $opengauss_exporter \
#        --log.level=debug --web.listen-address=:$exporter_port &
#    exporter_pid=$!
#    # shellcheck disable=SC2064
#    trap "docker-compose logs; docker-compose down ; docker-compose rm -v ; kill $exporter_pid; exit 1" EXIT INT TERM
#    wait_for_exporter
#
#    if ! wget -q -O - http://localhost:$exporter_port/metrics 1> "$METRICS_DIR/.metrics.replicated.$version.prom" ; then
#        echo "Failed on postgres $version (replicated $DOCKER_IMAGE)" 1>&2
#        exit 1
#    fi
#
#    kill $exporter_pid
#    docker-compose down
#    docker-compose rm -v
#    trap - EXIT INT TERM
#
#    cd "$old_pwd" || exit 1
}

# Start pulling the docker images in advance
for version in "${VERSIONS[@]}"; do
    docker pull "enmotech/opengauss:$version" > /dev/null &
done

for version in "${VERSIONS[@]}"; do
    echo "Testing openGauss version $version"
    smoke_test_opengauss "$version"
done