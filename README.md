# cloudshare

Program that watches a directory for new screenshots and recordings and uploads them to R2 bucket and copies the link into the keyboard automatically.

- [cloudshare.plist.template](./cloudshare.plist.template) can be used as the configuration for the LaunchAgent that runs the program in the background
  - The program requires environment variables that are specific in the .plist file.
