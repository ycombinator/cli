# Contributing

## Build & run the Xata CLI

If you want to build and run the CLI by yourself, the only dependency you need is Go.

* Compile and run

  ```
  go build -o xata .

  ./xata help
  ```

* If you like, you can add the `cli` path to your `$PATH`, so that
  you can call the CLI from everywhere. For this, run this command inside your cloned `cli` repo's root:

  ```
  export PATH=$(pwd):$PATH
  ```
  
  Alternatively, you can the following line to your `~/.zshrc` file to have it on your path each time you open your terminal:
  
  ```
  export PATH=/path/to/this/repo/root:$PATH
  ```
