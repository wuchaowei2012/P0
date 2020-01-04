# To compile and build the srunner program into a binary that you can run, 
# execute the three commands below 
# (these directions assume you have cloned this repo to $HOME/p0):

# export GOPATH="$GOPATH":/root/j/GitHub/P0
# export GOPATH=/root/j/GitHub/P0
rm srunner
go build github.com/cmu440/srunner 
./srunner& 
nc localhost 9999