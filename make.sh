#!/bin/bash

OPENALPR_INCLUDE_DIR=$(pwd)/../../openalpr # /home/ubuntu/openalpr/src/openalpr
OPENALPR_LIB_DIR=$(pwd)/../../build/openalpr # /home/ubuntu/openalpr/src/build/openalpr

export LD_LIBRARY_PATH=$LD_LIBRARY_PATH:.:${OPENALPR_LIB_DIR}

g++ -Wall -L${OPENALPR_LIB_DIR} -I${OPENALPR_INCLUDE_DIR} -shared -fPIC -o libopenalprgo.so openalprgo.cpp -lopenalpr

(cd openalpr && go install)

#go run main.go

go build -o felix

# -L = Search path for libraries
# -I = Search path for header files
# -shared = Builds dynamic library
# -fPIC only used for building shared libraries -> see man g++
# -o g++ -o target_name file_name: Compiles and links file_name and generates executable target file with target_name (or a.out by default).

#cgo pkg-config: openalpr
#cgo CFLAGS: -Wall -I/home/ubuntu/openalpr/src/openalpr
#cgo LDFLAGS: -L/home/ubuntu/openalpr/src/build/openalpr
#cgo LDLIBS: -lopenalpr