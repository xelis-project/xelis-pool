rm -r build
mkdir build

echo "Building Master"
cd cmd/master/
env CGO_ENABLED=0 go build -trimpath
strip ./master
cp ./master ../../build

echo "Building Slave"
cd ../slave/
env CGO_ENABLED=0 go build -trimpath
strip ./slave
cp ./slave ../../build/
