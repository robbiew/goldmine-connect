# This script is used to build the API project

#!/bin/bash

# Create build directory
if [ ! -d "bin" ]; then
    mkdir bin
fi

# Compile source files
go build .

# Move executable to bin directory
mv goldmine-connect bin/

echo "Build completed successfully!"
echo "Executable is located in bin/ directory"
