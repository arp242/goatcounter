# GoatCounter

This package contains the source of [GoatCounter's](https://www.goatcounter.com/) `count.js` and its corresponding TypeScript definitions.

## Installation

```bash
npm install goatcounter
```

## Usage

This package facilitates [self-hosting](https://www.goatcounter.com/help/countjs-host) and/or provides TypeScript definitions. It does not intend to be imported as a conventional module.

### Self-Hosting

1. Programmatically copy `./node_modules/goatcounter/public/count.js` to a public webroot directory during your application's build process.

2. Reference the script via relative URL:

    ```html
    <script
        data-goatcounter="https://MYCODE.goatcounter.com/count"
        async
        src="/count.js"
    ></script>
    ```

### TypeScript

To extend the global `Window` interface with a typed `goatcounter` property, configure TSConfig [`types`](https://www.typescriptlang.org/tsconfig/#types):

```json
{
    "compilerOptions": {
        "types": ["goatcounter"]
    }
}
```
