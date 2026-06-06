
# Git

```
git init;
git add .;

git commit -m "Initial commit: go-loghub-uuid implementation";

# Configurar autor
git config --global user.name patrickbrandao;
git config --global user.email patrickbrandao@gmail.com;

# Cria a tag (o -f foi usado para sobrescrever a antiga);
git tag -f v0.1.0;

# Envia a tag para o GitHub (o -f forcou a atualizacao remota)
git push -f origin v0.1.0;
git push origin v0.1.0;

# Atualiza a URL do repositorio remoto "origin" com a URL correta e completa
git remote set-url origin https://github.com/patrickbrandao/go-loghub-uuid.git;

git push origin main;
git push -u origin main;

```
