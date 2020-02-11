# Import
A microservice preparing and importing snippets and metadata files for use in the Taliesin app.

### Exposed REST API

**POST */createDB***  

**Request body**: Nothing expected for now. 

**Returned data**: a 200 status if everything went well or a 500 status if something was wrong with the error message as body.

**POST */upload***

**Request body**: multipart form data containing at key "file" the content of the snippet to import

**Returned data**: a 200 status if everything went well or a 4xx or 5xx status if something was wrong with the error message as body.

## Commits
The title of a commit must follow this pattern : \<type>(\<scope>): \<subject>

### Type
Commits must specify their type among the following:
* **build**: changes that affect the build system or external dependencies
* **docs**: documentation only changes
* **feat**: a new feature
* **fix**: a bug fix
* **perf**: a code change that improves performance
* **refactor**: modifications of code without adding features nor bugs (rename, white-space, etc.)
* **style**: CSS, layout modifications or console prints
* **test**: tests or corrections of existing tests
* **ci**: changes to our CI configuration


### Scope
Your commits name should also precise which part of the project they concern. You can do so by naming them using the following scopes:
* General
* Storage
* Communication
