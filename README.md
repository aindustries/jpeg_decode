### JPEG Decoder

This is a working demo of a jpeg decoder project. It takes as input the jpeg contained in this project, decodes it into a golang image struct, and then writes the image as a png. The png encode uses the go library encoder. The purpose of using a png output is to visually verify correctness. The projects from which this is extracted use an X11 viewer.  

This decoder is entirely from the JPEG spec: https://www.w3.org/Graphics/JPEG/itu-t81.pdf and no other jpeg decoding implementations. I made this to understand the image decode process. The goal of this project was learning, not performance. For instance, the discrete cosine transform used is in its most clear form but also its most inefficient. As such, this demo is very slow. 

### Usage

```go run main.go```

An output will appear in /tmp/out.png

### License

This is a working demo to be used as talking points. It is part of a larger set of projects that I'd like to present in a different way. As such, there is no license granted on this code and all rights are reserved. 
