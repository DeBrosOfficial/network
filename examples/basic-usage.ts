import { BaseModel, Model, Field, BelongsTo, HasMany } from '../src/framework';

// Example User model
@Model({ 
  scope: 'global',
  type: 'docstore',
  pinning: { strategy: 'fixed', factor: 2 }
})
export class User extends BaseModel {
  @Field({ type: 'string', required: true })
  username!: string;

  @Field({ type: 'string', required: true })
  email!: string;

  @Field({ type: 'string', required: false })
  bio?: string;

  @Field({ type: 'number', default: 0 })
  postCount!: number;

  @HasMany(Post, 'userId')
  posts!: Post[];
}

// Example Post model
@Model({ 
  scope: 'user',
  type: 'docstore',
  pinning: { strategy: 'popularity', factor: 3 }
})
export class Post extends BaseModel {
  @Field({ type: 'string', required: true })
  title!: string;

  @Field({ type: 'string', required: true })
  content!: string;

  @Field({ type: 'string', required: true })
  userId!: string;

  @Field({ type: 'boolean', default: true })
  isPublic!: boolean;

  @Field({ type: 'array', default: [] })
  tags!: string[];

  @BelongsTo(User, 'userId')
  author!: User;

  @HasMany(Comment, 'postId')
  comments!: Comment[];
}

// Example Comment model
@Model({ 
  scope: 'user',
  type: 'docstore'
})
export class Comment extends BaseModel {
  @Field({ type: 'string', required: true })
  content!: string;

  @Field({ type: 'string', required: true })
  userId!: string;

  @Field({ type: 'string', required: true })
  postId!: string;

  @BelongsTo(User, 'userId')
  author!: User;

  @BelongsTo(Post, 'postId')
  post!: Post;
}

// Example usage (this would work once database integration is complete)
async function exampleUsage() {
  try {
    // Create a new user
    const user = new User({
      username: 'john_doe',
      email: 'john@example.com',
      bio: 'A passionate developer'
    });

    // The decorators ensure validation
    await user.save(); // This will validate fields and run hooks

    // Create a post
    const post = new Post({
      title: 'My First Post',
      content: 'This is my first post using the DebrosFramework!',
      userId: user.id,
      tags: ['framework', 'orbitdb', 'ipfs']
    });

    await post.save();

    // Query posts (these methods will work once QueryExecutor is implemented)
    // const publicPosts = await Post
    //   .where('isPublic', '=', true)
    //   .load(['author'])
    //   .orderBy('createdAt', 'desc')
    //   .limit(10)
    //   .exec();

    console.log('Models created successfully!');
  } catch (error) {
    console.error('Error:', error);
  }
}

export { exampleUsage };