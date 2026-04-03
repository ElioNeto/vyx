cd core
go build -o ../vyx ./cmd/vyx
cd ../examples/hello-world
export JWT_SECRET=supersecret
../../vyx dev