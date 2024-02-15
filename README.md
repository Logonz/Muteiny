Windows only

![image](https://user-images.githubusercontent.com/8261057/188181297-8ffff80b-6d21-44a5-9c11-3bc962900919.png)

use build-windowless.sh to build it without a console window

To figure out mouse keys change the code to output it for you.

Run as administrator

```
Usage of muteiny.exe:
  -h value
        Alias of -holdtime (default 150)
  -holdtime value
        Specify the time in milliseconds to keep the mic open after release (default 150) (default 150)
  -k value
        Alias of -keybind
  -keybind value
        Specify keybind in format VK_A
  -md value
        Alias of -mousedown
  -mousedown value
        Specify mouse keybind in format 523 (down) !set both mouse up and down for it to work!
  -mouseup value
        Specify mouse keybind in format 524 (up) !set both mouse up and down for it to work!
  -mu value
        Alias of -mouseup
  -mousedata value
        Specify mouse data in format 131072(mouse3)/65536(mouse4), else all data is accepted
  -mdata
        Print mouse data
  -keybindmode
        Set the program to bind mode, this will not mute the mic but instead write the binds to the console/binds.log to help you find the correct VK/Mouse codes
```

`./Muteiny.exe -k VK_G -md 523 -mu 524`
`./Muteiny.exe -md 523 -mu 524 -h 450`
`./Muteiny.exe -md 523 -mu 524 -mdata 131072 -h 500`
