#!/usr/bin/env bash
set -Eeo pipefail 



# check to see if this file is being run or sourced from another script
_is_sourced() {
	[ "${#FUNCNAME[@]}" -ge 2 ] \
		&& [ "${FUNCNAME[0]}" = '_is_sourced' ] \
		&& [ "${FUNCNAME[1]}" = 'source' ]
}

_mtk_want_help() {
	local arg
	for arg; do
		case "$arg" in
			--help|-h|-v|--version)
				return 0
				;;
		esac
	done
	return 1
}

_main() {
	# if first arg looks like a flag, assume we want to run openGauss server
	if [ "${1:0:1}" = '-' ]; then
		set -- opengauss_exporter "$@"
	fi

	exec "$@"
}

if ! _is_sourced; then
	_main "$@"
fi
