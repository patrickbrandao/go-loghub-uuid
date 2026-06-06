
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

# Cria a tag (o -f foi usado para sobrescrever a antiga);
git tag -f v0.1.0;

# Envia a tag para o GitHub (o -f forcou a atualizacao remota)
git push -f origin v0.1.0;
git push origin v0.1.0;

```


```
git init;
git add .;







```

