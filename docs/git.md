
# Git

```
git init;

# Configurar autor
git config --global user.name patrickbrandao;
git config --global user.email patrickbrandao@gmail.com;

git branch -M main;
git remote add origin https://github.com/patrickbrandao/go-loghub-uuid.git;
git push -u origin main;

# Atualiza a URL do repositorio remoto "origin" com a URL correta e completa
git remote set-url origin https://github.com/patrickbrandao/go-loghub-uuid.git;

# Aplicar commit
git commit -m "Update";
git push -u origin main;

# Cria a tag anotada para a nova versão (tags publicadas são imutáveis;
# nunca sobrescreva tags com -f para não quebrar o sum.golang.org dos usuários)
git tag -a v0.3.0 -m "Release version 0.3.0";

# Envia a tag para o GitHub
git push origin v0.3.0;

gh release create v0.3.0 --title "v0.3.0" --generate-notes;

```


```
git init;
git add .;







```

